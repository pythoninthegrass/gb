package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitfield/script"
	"github.com/spf13/cobra"
)

type Config struct {
	RepoDir    string
	OutputDir  string
	MaxJobs    int
	Verbose    bool
	Force      bool
	Debug      bool
}

type Progress struct {
	Total     int64
	Current   int64
	Failed    int64
	FailedRepos []string
	mu        sync.Mutex
}

func (p *Progress) Increment() {
	atomic.AddInt64(&p.Current, 1)
}

func (p *Progress) AddFailed(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailedRepos = append(p.FailedRepos, name)
	atomic.AddInt64(&p.Failed, 1)
}

func (p *Progress) GetCounts() (int64, int64, int64) {
	return atomic.LoadInt64(&p.Current), atomic.LoadInt64(&p.Failed), p.Total
}

func (p *Progress) GetFailedRepos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]string, len(p.FailedRepos))
	copy(result, p.FailedRepos)
	return result
}

var config Config
var progress Progress

func main() {
	var rootCmd = &cobra.Command{
		Use:   "gb",
		Short: "Create and restore git bundles",
		Long: `A high-performance tool for creating and restoring git bundles with parallel processing.

Git bundles are portable archives containing git repository data that can be
used for backup, transfer, or distribution purposes. This tool automatically
discovers git repositories and processes them in parallel for optimal performance.

Default behavior (no command specified): Creates bundles of all repositories.

Use "gb [command] --help" for detailed information about each command.`,
		Version: "1.0.0",
	}

	// Initialize config with defaults
	initConfig()

	var backupCmd = &cobra.Command{
		Use:     "backup",
		Aliases: []string{"b"},
		Short:   "Create bundles of all repositories",
		Long: `Create git bundles for all repositories found in the source directory.

This command recursively searches for git repositories and creates compressed
bundle files that can be used for backup or transfer purposes.

Environment Variables:
  REPO_DIR          Source directory for repositories (default: ~/git)
  OUTPUT_DIR        Output directory for bundles (default: /tmp)
  MAX_JOBS          Maximum parallel jobs (default: auto-detect, max 8)

Examples:
  gb backup                              # Create bundles with defaults
  gb backup -v                           # Verbose output showing progress
  gb backup -x                           # Enable debug tracing
  REPO_DIR=/path/to/repos gb backup      # Custom source directory
  OUTPUT_DIR=/backups gb backup          # Custom output directory`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := backup(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	var restoreCmd = &cobra.Command{
		Use:     "restore [bundle-dir] [dest-dir]",
		Aliases: []string{"r"},
		Short:   "Restore bundles from directory",
		Long: `Restore git repositories from bundle files.

This command finds all .bundle files in the specified directory and clones
them to create working git repositories. If repositories already exist at
the destination, you will be prompted for confirmation unless --force is used.

Arguments:
  bundle-dir        Directory containing .bundle files (default: /tmp)
  dest-dir          Destination directory for repositories (default: ~/git)

Examples:
  gb restore                             # Restore from /tmp to ~/git
  gb restore -f                          # Force overwrite without confirmation
  gb restore -v                          # Verbose output showing progress
  gb restore /backups ~/restored        # Custom source and destination
  gb restore /backups ~/restored -f     # Custom paths with force overwrite`,
		Args:    cobra.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			bundleDir := config.OutputDir
			destDir := config.RepoDir

			if len(args) >= 1 {
				bundleDir = args[0]
			}
			if len(args) >= 2 {
				destDir = args[1]
			}

			if err := restore(bundleDir, destDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Add flags
	backupCmd.Flags().BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose output")
	backupCmd.Flags().BoolVarP(&config.Debug, "debug", "x", false, "Enable debug output")

	restoreCmd.Flags().BoolVarP(&config.Verbose, "verbose", "v", false, "Enable verbose output")
	restoreCmd.Flags().BoolVarP(&config.Debug, "debug", "x", false, "Enable debug output")
	restoreCmd.Flags().BoolVarP(&config.Force, "force", "f", false, "Force overwrite without confirmation")

	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)

	// Set backup as default command
	rootCmd.Run = backupCmd.Run

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initConfig() {
	// Get current user
	usr, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not get current user: %v\n", err)
		usr = &user.User{HomeDir: os.Getenv("HOME")}
	}

	// Set defaults
	config.RepoDir = getEnvOrDefault("REPO_DIR", filepath.Join(usr.HomeDir, "git"))
	config.OutputDir = getEnvOrDefault("OUTPUT_DIR", "/tmp")

	// Set max jobs
	if maxJobsEnv := os.Getenv("MAX_JOBS"); maxJobsEnv != "" {
		fmt.Sscanf(maxJobsEnv, "%d", &config.MaxJobs)
	} else {
		config.MaxJobs = min(runtime.NumCPU(), 8)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func logVerbose(format string, args ...interface{}) {
	if config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func backup() error {
	fmt.Println("Starting git bundling!")
	logVerbose("Repository directory: %s", config.RepoDir)
	logVerbose("Output directory: %s", config.OutputDir)

	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Find all git repositories using find command
	gitDirs, err := script.Exec(fmt.Sprintf("find %s -maxdepth 2 -name '*.git' -type d",
		strconv.Quote(config.RepoDir))).Slice()
	if err != nil {
		return fmt.Errorf("failed to find git repositories: %w", err)
	}

	progress.Total = int64(len(gitDirs))
	logVerbose("Found %d repositories", progress.Total)

	if progress.Total == 0 {
		fmt.Printf("No repositories found in %s\n", config.RepoDir)
		return nil
	}

	logVerbose("Using %d parallel jobs", config.MaxJobs)

	// Start progress monitoring
	done := make(chan bool)
	go monitorProgress(done)

	// Process repositories in parallel
	jobs := make(chan string, len(gitDirs))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < config.MaxJobs; i++ {
		wg.Add(1)
		go bundleWorker(jobs, &wg)
	}

	// Send jobs
	for _, gitDir := range gitDirs {
		jobs <- gitDir
	}
	close(jobs)

	// Wait for completion
	wg.Wait()
	done <- true

	// Print final results
	current, failed, total := progress.GetCounts()
	failedRepos := progress.GetFailedRepos()

	fmt.Printf("\rBundling repositories: %d/%d\n", current, total)

	// Print failed repositories
	for _, failed := range failedRepos {
		fmt.Printf("Failed to bundle '%s'\n", failed)
	}
	if len(failedRepos) > 0 {
		fmt.Println()
	}

	// Print summary
	success := total - failed
	fmt.Printf("Summary:\n")
	fmt.Printf("    %-22s %d\n", "Total repositories:", total)
	fmt.Printf("    %-22s %d\n", "Successfully bundled:", success)
	fmt.Printf("    %-22s %d\n", "Failed:", failed)
	fmt.Printf("    %-22s %d\n", "Parallel jobs used:", config.MaxJobs)
	fmt.Printf("\n")

	fmt.Printf("Finished!\n\nTo extract the bundles, use the following command:\n    git clone <bundle-file> <destination-directory>\n")

	// Open output directory (platform-specific)
	openOutputDir()

	return nil
}

func bundleWorker(jobs <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for gitDir := range jobs {
		repoDir := filepath.Dir(gitDir)
		repoName := filepath.Base(repoDir)
		bundlePath := filepath.Join(config.OutputDir, repoName+".bundle")

		logVerbose("Processing repository: %s", repoName)

		// Create bundle
		_, err := script.Exec(fmt.Sprintf("git -C %s bundle create %s --all",
			strconv.Quote(repoDir), strconv.Quote(bundlePath))).Stdout()

		if err != nil {
			logVerbose("Failed to bundle %s: %v", repoName, err)
			progress.AddFailed(repoName)
		}

		progress.Increment()
	}
}

func monitorProgress(done <-chan bool) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			current, _, total := progress.GetCounts()
			fmt.Printf("\rBundling repositories: %d/%d", current, total)
		}
	}
}

func openOutputDir() {
	var cmd string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
	case "darwin":
		cmd = "open"
	default:
		fmt.Printf("Please open the output directory manually: %s\n", config.OutputDir)
		return
	}

	script.Exec(fmt.Sprintf("%s %s", cmd, strconv.Quote(config.OutputDir))).Wait()
}

func restore(bundleDir, destDir string) error {
	fmt.Println("Starting git bundle restoration!")
	logVerbose("Bundle directory: %s", bundleDir)
	logVerbose("Destination directory: %s", destDir)

	// Validate bundle directory exists
	if _, err := os.Stat(bundleDir); os.IsNotExist(err) {
		return fmt.Errorf("bundle directory '%s' does not exist", bundleDir)
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Find all bundle files using filepath.Glob
	bundlePattern := filepath.Join(bundleDir, "*.bundle")
	bundleFiles, err := filepath.Glob(bundlePattern)
	if err != nil {
		return fmt.Errorf("failed to find bundle files: %w", err)
	}

	if len(bundleFiles) == 0 {
		fmt.Printf("No bundle files found in %s\n", bundleDir)
		return nil
	}

	// Check for existing repositories
	var existingRepos []string
	for _, bundleFile := range bundleFiles {
		bundleName := strings.TrimSuffix(filepath.Base(bundleFile), ".bundle")
		destPath := filepath.Join(destDir, bundleName)
		if _, err := os.Stat(destPath); err == nil {
			existingRepos = append(existingRepos, bundleName)
		}
	}

	// Ask for confirmation if repositories exist
	if len(existingRepos) > 0 && !config.Force {
		fmt.Println("Warning: The following repositories already exist and will be overwritten:")
		for _, repo := range existingRepos {
			fmt.Printf("  - %s\n", repo)
		}
		fmt.Println()

		fmt.Print("Continue and overwrite existing repositories? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if !strings.HasPrefix(strings.ToLower(response), "y") {
			fmt.Println("Restore cancelled by user")
			return nil
		}
		fmt.Println()
	} else if len(existingRepos) > 0 {
		logVerbose("Force flag enabled, proceeding without confirmation")
	}

	// Restore bundles
	var totalCount, successCount, failedCount int64
	totalCount = int64(len(bundleFiles))

	for _, bundleFile := range bundleFiles {
		bundleName := strings.TrimSuffix(filepath.Base(bundleFile), ".bundle")
		destPath := filepath.Join(destDir, bundleName)

		fmt.Printf("Restoring %s...", bundleName)

		// Remove existing directory if it exists
		if _, err := os.Stat(destPath); err == nil {
			logVerbose("Removing existing directory: %s", destPath)
			if err := os.RemoveAll(destPath); err != nil {
				fmt.Printf(" ✗\n")
				logVerbose("Failed to remove existing directory %s: %v", destPath, err)
				failedCount++
				continue
			}
		}

		// Clone from bundle
		_, err := script.Exec(fmt.Sprintf("git clone %s %s",
			strconv.Quote(bundleFile), strconv.Quote(destPath))).Stdout()

		if err != nil {
			fmt.Printf(" ✗\n")
			logVerbose("Failed to clone %s to %s: %v", bundleFile, destPath, err)
			failedCount++
		} else {
			fmt.Printf(" ✓\n")
			successCount++
		}
	}

	// Print summary
	fmt.Printf("\nRestore Summary:\n")
	fmt.Printf("    %-22s %d\n", "Total bundles:", totalCount)
	fmt.Printf("    %-22s %d\n", "Successfully restored:", successCount)
	fmt.Printf("    %-22s %d\n", "Failed:", failedCount)
	fmt.Printf("\n")

	fmt.Printf("Finished restoring bundles to %s\n", destDir)
	return nil
}
