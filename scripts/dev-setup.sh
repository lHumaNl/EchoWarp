#!/bin/bash
# Development environment setup script for EchoWarp
# Checks and installs required dependencies for development

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Minimum required Go version
MIN_GO_VERSION="1.22"

# Function to print colored output
print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Usage function
usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Development environment setup script for EchoWarp.

Options:
  -h, --help     Show this help message
  -y, --yes      Automatic yes to prompts

This script will:
  1. Check Go version (>= $MIN_GO_VERSION)
  2. Check CGo dependencies (libopus)
  3. Install golangci-lint if not present
  4. Install pre-commit if not present
  5. Download Go modules

EOF
    exit 0
}

# Parse arguments
AUTO_YES=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -y|--yes)
            AUTO_YES=true
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

echo "=== EchoWarp Development Environment Setup ==="
echo ""

# Check Go version
echo "Checking Go version..."
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go >= $MIN_GO_VERSION"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

MIN_MAJOR=$(echo "$MIN_GO_VERSION" | cut -d. -f1)
MIN_MINOR=$(echo "$MIN_GO_VERSION" | cut -d. -f2)

if [ "$GO_MAJOR" -lt "$MIN_MAJOR" ] || ([ "$GO_MAJOR" -eq "$MIN_MAJOR" ] && [ "$GO_MINOR" -lt "$MIN_MINOR" ]); then
    print_error "Go version $GO_VERSION is too old. Minimum required: $MIN_GO_VERSION"
    exit 1
fi

print_success "Go version: $GO_VERSION"

# Check CGo dependencies (libopus)
echo ""
echo "Checking CGo dependencies..."

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux - check for libopus-dev
    if pkg-config --exists opus 2>/dev/null || [ -f /usr/include/opus/opus.h ] || [ -f /usr/local/include/opus/opus.h ]; then
        print_success "libopus found"
    else
        print_error "libopus-dev not found. Install with:"
        print_warning "  Ubuntu/Debian: sudo apt-get install libopus-dev"
        print_warning "  Fedora/RHEL:   sudo dnf install opus-devel"
        exit 1
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS - check for libopus via Homebrew
    if brew list opus &>/dev/null || pkg-config --exists opus 2>/dev/null || [ -f /usr/local/include/opus/opus.h ] || [ -f /opt/homebrew/include/opus/opus.h ]; then
        print_success "libopus found"
    else
        print_error "libopus not found. Install with:"
        print_warning "  brew install opus"
        exit 1
    fi
else
    print_warning "Unknown OS: $OSTYPE. Skipping libopus check."
fi

# Check CGo compiler
if ! command -v cc &> /dev/null; then
    print_error "C compiler not found. CGo requires a C compiler."
    print_warning "  Linux: install gcc or clang"
    print_warning "  macOS: install Xcode Command Line Tools: xcode-select --install"
    exit 1
fi
print_success "C compiler found: $(cc --version | head -n1)"

# Install golangci-lint
echo ""
echo "Checking golangci-lint..."
if ! command -v golangci-lint &> /dev/null; then
    print_warning "golangci-lint not found. Installing..."
    
    if ! curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v1.62.0; then
        print_error "Failed to install golangci-lint"
        exit 1
    fi
    
    print_success "golangci-lint installed successfully"
else
    print_success "golangci-lint found: $(golangci-lint --version | head -n1)"
fi

# Install pre-commit
echo ""
echo "Checking pre-commit..."
if ! command -v pre-commit &> /dev/null; then
    print_warning "pre-commit not found."
    
    if [[ "$AUTO_YES" != "true" ]]; then
        read -p "Install pre-commit? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_warning "Skipping pre-commit installation"
        else
            INSTALL_PRECOMMIT=true
        fi
    else
        INSTALL_PRECOMMIT=true
    fi
    
    if [ "$INSTALL_PRECOMMIT" = true ]; then
        if command -v pip3 &> /dev/null; then
            pip3 install pre-commit
            print_success "pre-commit installed successfully"
        elif command -v pip &> /dev/null; then
            pip install pre-commit
            print_success "pre-commit installed successfully"
        else
            print_error "pip/pip3 not found. Cannot install pre-commit"
            print_warning "Install with: pip install pre-commit or brew install pre-commit"
        fi
    fi
else
    print_success "pre-commit found: $(pre-commit --version 2>&1 | head -n1)"
fi

# Download Go modules
echo ""
echo "Downloading Go modules..."
if ! go mod download; then
    print_error "Failed to download Go modules"
    exit 1
fi
print_success "Go modules downloaded"

# Success message
echo ""
echo -e "${GREEN}=== Setup Complete ===${NC}"
echo ""
echo "Next steps:"
echo "  1. Run 'make build' to build the project"
echo "  2. Run 'make test' to run tests"
echo "  3. Run 'make lint' to check code quality"
echo "  4. If using pre-commit, run 'pre-commit install' to set up git hooks"
echo ""
echo "For more information, see README.md and CONTRIBUTING.md"
