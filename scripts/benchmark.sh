#!/bin/bash
# Run benchmarks and save results

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Benchmark settings
BENCHMARK_DIR="benchmarks"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_FILE="$BENCHMARK_DIR/${TIMESTAMP}.txt"

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

Run benchmarks and save output to $BENCHMARK_DIR/ directory.

Options:
  -h, --help     Show this help message
  -o, --output   Specify custom output file

This script will:
  1. Create $BENCHMARK_DIR/ directory if it doesn't exist
  2. Run 'go test -bench=. -benchmem ./...'
  3. Save output to $BENCHMARK_DIR/TIMESTAMP.txt
  4. Display summary

EOF
    exit 0
}

# Parse arguments
CUSTOM_OUTPUT=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -o|--output)
            CUSTOM_OUTPUT="$2"
            shift 2
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

echo "=== EchoWarp Benchmark Runner ==="
echo ""

# Set output file
if [ -n "$CUSTOM_OUTPUT" ]; then
    OUTPUT_FILE="$CUSTOM_OUTPUT"
    # Ensure directory exists
    mkdir -p "$(dirname "$OUTPUT_FILE")"
else
    # Create benchmarks directory
    mkdir -p "$BENCHMARK_DIR"
fi

echo "Output file: $OUTPUT_FILE"
echo ""
echo "Running benchmarks..."
echo ""

# Run benchmarks and capture output
if ! go test -bench=. -benchmem ./... 2>&1 | tee "$OUTPUT_FILE"; then
    print_error "Benchmarks failed"
    exit 1
fi

echo ""
print_success "Benchmarks completed successfully"
print_success "Results saved to: $OUTPUT_FILE"

# Show summary
echo ""
echo "=== Summary ==="
echo "Benchmark file: $OUTPUT_FILE"
echo "File size: $(du -h "$OUTPUT_FILE" | cut -f1)"
echo "Timestamp: $TIMESTAMP"
echo ""
echo "To view results:"
echo "  cat $OUTPUT_FILE"
echo "  less $OUTPUT_FILE"
