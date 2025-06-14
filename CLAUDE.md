# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

The project uses Task (taskfile) for build automation:

- `task` - Show all available tasks
- `task build:all` - Build for all platforms (Linux/macOS amd64/arm64)  
- `task build:clean` - Remove build artifacts
- `task build:watch` - Watch for changes and rebuild
- `task build` - Build for current platform (output: bin/gb)
- `task go:deps` - Install/update Go dependencies
- `task ko:build-platforms` - Build multi-platform container images (linux/amd64,linux/arm64)
- `task ko:build-push` - Build and push container image to registry
- `task ko:build` - Build container image locally with ko
- `task test:backup` - Test backup functionality with verbose output
- `task test:restore` - Test restore functionality with verbose output
- `task test` - Run all tests

## Architecture

This is a single-file Go CLI application (`main.go`) built with Cobra and the bitfield/script library that creates and restores git bundles:

- **Core functionality**: Parallel processing of git repositories to create compressed bundle files for backup/transfer
- **Commands**: 
  - `backup` (default): Creates bundles from all git repos in a directory tree
  - `restore`: Restores repositories from bundle files
- **Key features**:
  - Multi-threaded processing with configurable job count (default: CPU cores, max 8)
  - Progress monitoring with real-time updates
  - Error tracking and reporting
  - Environment variable configuration (REPO_DIR, OUTPUT_DIR, MAX_JOBS)
  - Cross-platform directory opening after completion
- **Libraries**:
  - [cobra](https://github.com/spf13/cobra): CLI framework for commands, flags, and help
  - [bitfield/script](https://github.com/bitfield/script): Shell command execution pipeline (replaces exec.Command for cleaner shell operations)
- **Containerization**:
  - [ko](https://ko.build) (container image builder) configuration uses `KO_DOCKER_REPO` variable combining registry, org, and binary name from taskfile.yml variables.

## Configuration

Environment variables:

- `REPO_DIR`: Source directory for repositories (default: ~/git)
- `OUTPUT_DIR`: Output directory for bundles (default: /tmp)  
- `MAX_JOBS`: Maximum parallel jobs (default: auto-detect, max 8)

Build variables in taskfile.yml control versioning and binary naming.
