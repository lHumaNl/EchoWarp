#!/bin/bash
# Generate self-signed TLS certificates for testing

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Certificate settings
CERT_DIR="certs"
CERT_FILE="server.crt"
KEY_FILE="server.key"
VALIDITY_DAYS=365

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

Generate self-signed TLS certificates for local testing.

Options:
  -h, --help     Show this help message
  -f, --force    Force overwrite existing certificates

This script generates:
  - $CERT_DIR/$CERT_FILE  (server certificate)
  - $CERT_DIR/$KEY_FILE   (private key)

Certificate properties:
  - Valid for: localhost, 127.0.0.1
  - Validity: $VALIDITY_DAYS days

Note: Add 'certs/' to .gitignore to avoid committing test certificates.

EOF
    exit 0
}

# Parse arguments
FORCE=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            ;;
    esac
done

echo "=== TLS Certificate Generator ==="
echo ""

# Check if openssl is available
if ! command -v openssl &> /dev/null; then
    print_error "OpenSSL is not installed. Please install it first."
    exit 1
fi

# Check if certificates already exist
if [ -f "$CERT_DIR/$CERT_FILE" ] || [ -f "$CERT_DIR/$KEY_FILE" ]; then
    if [ "$FORCE" = false ]; then
        print_warning "Certificates already exist in $CERT_DIR/"
        print_warning "Use -f or --force to overwrite"
        exit 1
    fi
    print_warning "Overwriting existing certificates..."
fi

# Create certs directory
mkdir -p "$CERT_DIR"

# Generate private key and certificate
echo ""
echo "Generating certificate..."

openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$CERT_DIR/$KEY_FILE" \
    -out "$CERT_DIR/$CERT_FILE" \
    -days "$VALIDITY_DAYS" \
    -subj "/C=US/ST=State/L=City/O=EchoWarp/OU=Development/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
    2>/dev/null

# Set proper permissions
chmod 600 "$CERT_DIR/$KEY_FILE"
chmod 644 "$CERT_DIR/$CERT_FILE"

print_success "Certificate generated: $CERT_DIR/$CERT_FILE"
print_success "Private key generated: $CERT_DIR/$KEY_FILE"

# Reminder about .gitignore
echo ""
print_warning "Remember to add 'certs/' to .gitignore"
echo "  echo 'certs/' >> .gitignore"

echo ""
echo -e "${GREEN}=== Certificate Generation Complete ===${NC}"
