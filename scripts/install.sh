#!/usr/bin/env bash

# shellcheck disable=SC2155

set -euo pipefail

cat << 'DESCRIPTION' >/dev/null
Install script for gb (git bundle tool)

Refactored from: https://gist.github.com/pythoninthegrass/cc3163147b5f838f1582c364d8e33087
DESCRIPTION

# Configuration - hardcoded for this repo
readonly USER_NAME="pythoninthegrass"
readonly REPO_NAME="gb"
readonly BASE_URL="https://github.com/${USER_NAME}/${REPO_NAME}/releases"

# Environment variable overrides with defaults
PKG_PATH="${PKG_PATH:-${HOME}/.local/bin}"
PKG_TYPE="${PKG_TYPE:-tar.gz}"
PKG_NAME="${PKG_NAME:-${REPO_NAME}}"
FORCE="${FORCE:-false}"

# System detection
readonly DISTRO=$(uname)  # Linux / Darwin
readonly ARCH=$(uname -m) # x86_64 / aarch64 / arm64

help() {
    cat << EOF
USAGE
    $(basename "$0") [options]

DESCRIPTION
    Download and install the latest $PKG_NAME release from GitHub.

OPTIONS
    -h, --help      Show this help message and exit
    -p, --path      Installation path (default: ~/.local/bin)
    -t, --type      Package type: tar.gz, zip (default: tar.gz)
    -f, --force     Force overwrite existing installation
    -v, --version   Show version information

ENVIRONMENT VARIABLES
    PKG_PATH 		Package installation path (default: ~/.local/bin)
    PKG_TYPE     	Package type (default: tar.gz)
    PKG_NAME     	Binary name (default: repo name)
    FORCE        	Force overwrite (true/false, default: false)

EXAMPLES
    $(basename "$0")                    		# Install to ~/.local/bin
    $(basename "$0") -p /usr/local/bin  		# Install to custom path
    $(basename "$0") -t zip             		# Install .zip package
    PKG_PATH=/opt/bin $(basename "$0")  		# Use environment variable
EOF
}

version() {
    echo "$PKG_NAME installer v0.1.0"
    echo "Repository: ${BASE_URL}"
}

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

error() {
    log "ERROR: $*"
    exit 1
}

check_dependencies() {
    local deps=("curl" "jq")

    # Add unzip dependency if using zip packages
    if [[ "$PKG_TYPE" == "zip" ]]; then
        deps+=("unzip")
    fi

    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            error "Required dependency '$dep' not found. Please install it first."
        fi
    done
}

get_latest_release_url() {
    local api_url="https://api.github.com/repos/${USER_NAME}/${REPO_NAME}/releases/latest"

    log "Fetching latest release information..."

    # Get release info and extract download URLs
    local release_data
    release_data=$(curl -s "$api_url") || error "Failed to fetch release information"

    # Extract assets and filter by architecture and package type
    local pkg_urls
    pkg_urls=$(echo "$release_data" | jq -r '.assets[].browser_download_url' | grep -E "\.(${PKG_TYPE//,/|})$") || {
        error "No packages found for type: $PKG_TYPE"
    }

    # Filter by architecture
    local filtered_url=""
    case "$ARCH" in
        x86_64|amd64)
            filtered_url=$(echo "$pkg_urls" | grep -E "(x86_64|amd64)" | head -1)
            ;;
        aarch64|arm64)
            filtered_url=$(echo "$pkg_urls" | grep -E "(aarch64|arm64)" | head -1)
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    # Filter by OS if needed
    case "$DISTRO" in
        Darwin)
            filtered_url=$(echo "$filtered_url" | grep -i darwin || echo "$filtered_url")
            ;;
        Linux)
            filtered_url=$(echo "$filtered_url" | grep -i linux || echo "$filtered_url")
            ;;
    esac

    if [[ -z "$filtered_url" ]]; then
        log "Available packages:"
        echo "$pkg_urls"
        error "No package found for $DISTRO $ARCH with type $PKG_TYPE"
    fi

    echo "$filtered_url"
}

download_package() {
    local url="$1"
    local output_path="$2"

    log "Downloading from: $url"
    log "Saving to: $output_path"

    curl -L -o "$output_path" "$url" || error "Failed to download package"

    log "Download completed: $(basename "$output_path")"
}

install_package() {
    local pkg_file="$1"
    local install_path="$2"

    # Create installation directory
    mkdir -p "$install_path"

    case "$PKG_TYPE" in
        tar.gz)
            log "Extracting tar.gz package..."
            local temp_dir
            temp_dir=$(mktemp -d)
            tar -xzf "$pkg_file" -C "$temp_dir" || error "Failed to extract package"

            # Find the binary (look for package name without extension)
            local binary_path
            binary_path=$(find "$temp_dir" -name "$PKG_NAME" -type f | head -1)

            if [[ -z "$binary_path" ]]; then
                error "Could not find '$PKG_NAME' binary in extracted package"
            fi

            # Check if binary already exists
            local target_path="${install_path}/${PKG_NAME}"
            if [[ -f "$target_path" ]] && [[ "$FORCE" != "true" ]]; then
                error "Binary already exists at $target_path. Use --force to overwrite."
            fi

            cp "$binary_path" "$target_path" || error "Failed to install binary"
            chmod +x "$target_path"

            # Cleanup
            rm -rf "$temp_dir"
            log "Installed $PKG_NAME to: $target_path"
            ;;
        zip)
            log "Extracting zip package..."
            local temp_dir
            temp_dir=$(mktemp -d)

            # Check if unzip is available
            if ! command -v unzip &> /dev/null; then
                error "unzip command not found. Please install unzip to handle .zip packages."
            fi

            unzip -q "$pkg_file" -d "$temp_dir" || error "Failed to extract zip package"

            # Find the binary (look for package name without extension)
            local binary_path
            binary_path=$(find "$temp_dir" -name "$PKG_NAME" -type f | head -1)

            if [[ -z "$binary_path" ]]; then
                error "Could not find '$PKG_NAME' binary in extracted package"
            fi

            # Check if binary already exists
            local target_path="${install_path}/${PKG_NAME}"
            if [[ -f "$target_path" ]] && [[ "$FORCE" != "true" ]]; then
                error "Binary already exists at $target_path. Use --force to overwrite."
            fi

            cp "$binary_path" "$target_path" || error "Failed to install binary"
            chmod +x "$target_path"

            # Cleanup
            rm -rf "$temp_dir"
            log "Installed $PKG_NAME to: $target_path"
            ;;
        *)
            error "Unsupported package type: $PKG_TYPE"
            ;;
    esac
}

cleanup() {
    if [[ -n "${temp_file:-}" ]] && [[ -f "$temp_file" ]]; then
        rm -f "$temp_file"
    fi
}

main() {
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                help
                exit 0
                ;;
            -v|--version)
                version
                exit 0
                ;;
            -p|--path)
                PKG_PATH="$2"
                shift 2
                ;;
            -t|--type)
                PKG_TYPE="$2"
                shift 2
                ;;
            -f|--force)
                FORCE="true"
                shift
                ;;
            *)
                error "Unknown option: $1. Use --help for usage information."
                ;;
        esac
    done

    # Validate package type
    case "$PKG_TYPE" in
        tar.gz|zip)
            ;;
        *)
            error "Invalid package type: $PKG_TYPE. Supported types: tar.gz, zip"
            ;;
    esac

    log "Starting $PKG_NAME installation..."
    log "System: $DISTRO $ARCH"
    log "Package type: $PKG_TYPE"
    log "Installation path: $PKG_PATH"

    # Setup cleanup trap
    trap cleanup EXIT

    # Check dependencies
    check_dependencies

    # Get latest release URL
    local pkg_url
    pkg_url=$(get_latest_release_url)

    # Download package
    temp_file=$(mktemp)
    download_package "$pkg_url" "$temp_file"

    # Install package
    install_package "$temp_file" "$PKG_PATH"

    # Verify installation
    if [[ "$PKG_TYPE" == "tar.gz" || "$PKG_TYPE" == "zip" ]]; then
        local installed_binary="${PKG_PATH}/${PKG_NAME}"
        if [[ -x "$installed_binary" ]]; then
            log "Installation successful!"
            log "Binary location: $installed_binary"

            # Check if PATH includes the installation directory
            if [[ ":$PATH:" != *":$PKG_PATH:"* ]]; then
                log "NOTE: Add $PKG_PATH to your PATH to use '$PKG_NAME' from anywhere:"
                log "  export PATH=\"\$PATH:$PKG_PATH\""
            fi

            # Show version
            log "Installed version:"
            "$installed_binary" --version || true
        else
            error "Installation verification failed"
        fi
    else
        log "Installation completed successfully!"
        log "You can now use the '$PKG_NAME' command."
    fi
}

# Only run main if script is executed directly (not sourced)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
