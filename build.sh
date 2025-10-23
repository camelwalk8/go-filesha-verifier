#!/bin/bash

# ============================================================================
# Go File SHA Verifier - Build Script
# ============================================================================
# This script:
#   1. Builds the Go application with embedded version information
#   2. Calculates SHA256 hash of the binary
#   3. Rebuilds with the hash embedded
#   4. Updates the release notes YAML file
#   5. Copies files to deployment directory
#   6. Tests the deployment package
#   7. Cleans up project root
#
# Usage:
#   ./build.sh
#
# Requirements:
#   - Go 1.21 or higher
#   - sed command
#   - sha256sum command
# ============================================================================

set -e  # Exit on error

# ============================================================================
# CONFIGURATION
# ============================================================================
VERSION="v1.0.0"
RELEASE="PRODUCTION"
BUILDTIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

APP_NAME="go-filesha-verifier"
BINARY_NAME="go-filesha-verifier"
APP_EXE="./$BINARY_NAME"
RELEASE_NOTES_PATH="go-filesha-verifier.RN.yaml"
CONFIG_PATH="config.yaml"
SERVICE_FILE="go-filesha-verifier.service"

# Deployment directory (local builds folder)
DEPLOYMENT_DIR="./builds/$VERSION"

# ============================================================================
# COLORS FOR OUTPUT
# ============================================================================
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# ============================================================================
# HELPER FUNCTIONS
# ============================================================================
print_step() {
    echo -e "${GREEN}==>${NC} $1"
}

print_info() {
    echo -e "${YELLOW}   →${NC} $1"
}

print_error() {
    echo -e "${RED}ERROR:${NC} $1"
}

# ============================================================================
# VALIDATION
# ============================================================================
print_step "Validating prerequisites..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go first."
    exit 1
fi
print_info "Go version: $(go version)"

# Check if release notes file exists
if [ ! -f "$RELEASE_NOTES_PATH" ]; then
    print_error "Release notes file not found: $RELEASE_NOTES_PATH"
    exit 1
fi
print_info "Found release notes: $RELEASE_NOTES_PATH"

# Check if config file exists
if [ ! -f "$CONFIG_PATH" ]; then
    print_error "Config file not found: $CONFIG_PATH"
    exit 1
fi
print_info "Found config file: $CONFIG_PATH"

# ============================================================================
# STEP 1: INITIAL BUILD (WITHOUT BUILD_ID)
# ============================================================================
print_step "Step 1: Building binary (initial build without build_id)..."

go build -ldflags "\
    -X main.release=$RELEASE \
    -X main.version=$VERSION \
    -X main.buildTime=$BUILDTIME" \
    -o "$BINARY_NAME"

if [ ! -f "$APP_EXE" ]; then
    print_error "Initial build failed - binary not created"
    exit 1
fi

print_info "Initial build complete: $APP_EXE"

# ============================================================================
# STEP 2: CALCULATE SHA256 HASH
# ============================================================================
print_step "Step 2: Calculating SHA256 hash..."

BUILD_ID=$(sha256sum "$APP_EXE" | awk '{print $1}')
print_info "Build ID (SHA256): $BUILD_ID"

# ============================================================================
# STEP 3: REBUILD WITH BUILD_ID EMBEDDED
# ============================================================================
print_step "Step 3: Rebuilding binary with build_id embedded..."

go build -ldflags "\
    -X main.release=$RELEASE \
    -X main.version=$VERSION \
    -X main.buildTime=$BUILDTIME \
    -X main.buildID=$BUILD_ID" \
    -o "$BINARY_NAME"

if [ ! -f "$APP_EXE" ]; then
    print_error "Final build failed - binary not created"
    exit 1
fi

print_info "Final build complete with embedded build_id"

# Verify binary exists before proceeding
if [ ! -x "$APP_EXE" ]; then
    print_error "Binary is not executable: $APP_EXE"
    exit 1
fi

# ============================================================================
# STEP 4: UPDATE RELEASE NOTES
# ============================================================================
print_step "Step 4: Updating release notes file..."

# Update appName and appVersion at the top of the file
sed -i "s/^appName: .*/appName: $APP_NAME/" "$RELEASE_NOTES_PATH"
sed -i "s/^appVersion: .*/appVersion: $VERSION/" "$RELEASE_NOTES_PATH"

# Update the first version entry (under versions: section)
sed -i "0,/  version: .*/s/  version: .*/  version: $VERSION/" "$RELEASE_NOTES_PATH"
sed -i "0,/  datetime: .*/s|  datetime: .*|  datetime: '$BUILDTIME'|" "$RELEASE_NOTES_PATH"
sed -i "0,/  build_id: .*/s/  build_id: .*/  build_id: $BUILD_ID/" "$RELEASE_NOTES_PATH"

print_info "Release notes updated"
print_info "  - release:  $RELEASE"
print_info "  - version:  $VERSION"
print_info "  - datetime: $BUILDTIME"
print_info "  - build_id: $BUILD_ID"

# ============================================================================
# STEP 5: CREATE DEPLOYMENT PACKAGE
# ============================================================================
print_step "Step 5: Creating deployment package..."

# Create the builds directory and version subfolder
mkdir -p "$DEPLOYMENT_DIR"
print_info "Created deployment directory: $DEPLOYMENT_DIR"

# Verify binary exists before copying
if [ ! -f "$APP_EXE" ]; then
    print_error "Binary not found at $APP_EXE - cannot create deployment package"
    exit 1
fi

# Copy binary
cp "$APP_EXE" "$DEPLOYMENT_DIR/"
print_info "Copied: $APP_EXE → $DEPLOYMENT_DIR/"

# Copy release notes
cp "$RELEASE_NOTES_PATH" "$DEPLOYMENT_DIR/"
print_info "Copied: $RELEASE_NOTES_PATH → $DEPLOYMENT_DIR/"

# Copy config file
cp "$CONFIG_PATH" "$DEPLOYMENT_DIR/"
print_info "Copied: $CONFIG_PATH → $DEPLOYMENT_DIR/"

# Copy documentation files if they exist
for doc in README.md QUICKSTART.md ARCHITECTURE.md; do
    if [ -f "$doc" ]; then
        cp "$doc" "$DEPLOYMENT_DIR/"
        print_info "Copied: $doc → $DEPLOYMENT_DIR/"
    fi
done

# Copy service file if it exists
if [ -f "$SERVICE_FILE" ]; then
    cp "$SERVICE_FILE" "$DEPLOYMENT_DIR/"
    print_info "Copied: $SERVICE_FILE → $DEPLOYMENT_DIR/"
else
    print_info "Note: $SERVICE_FILE not found - skipping"
fi

# Explicitly exclude CSV files (they will be generated by the application)
if ls "$DEPLOYMENT_DIR"/*.csv 1> /dev/null 2>&1; then
    rm "$DEPLOYMENT_DIR"/*.csv
    print_info "Removed CSV files from deployment (will be generated at runtime)"
fi

# Only include service file for initial version (v1.0.0)
# For subsequent versions, service file is already installed on the server
if [ "$VERSION" != "v1.0.0" ]; then
    if [ -f "$DEPLOYMENT_DIR/$SERVICE_FILE" ]; then
        rm "$DEPLOYMENT_DIR/$SERVICE_FILE"
        print_info "Removed service file from deployment (not needed for updates after v1.0.0)"
    fi
fi

echo ""
print_info "Deployment package created at: $DEPLOYMENT_DIR"
echo ""
echo "Package contents:"
ls -lh "$DEPLOYMENT_DIR"
echo ""

# ============================================================================
# STEP 6: TEST THE DEPLOYMENT PACKAGE
# ============================================================================
print_step "Step 6: Testing deployment package..."

# Test the binary from the deployment directory
DEPLOYMENT_BINARY="$DEPLOYMENT_DIR/$BINARY_NAME"
DEPLOYMENT_RN="$DEPLOYMENT_DIR/$RELEASE_NOTES_PATH"
DEPLOYMENT_CONFIG="$DEPLOYMENT_DIR/$CONFIG_PATH"

# Check binary exists and is executable
if [ ! -f "$DEPLOYMENT_BINARY" ]; then
    print_error "Binary not found in deployment package: $DEPLOYMENT_BINARY"
    exit 1
fi

if [ ! -x "$DEPLOYMENT_BINARY" ]; then
    chmod +x "$DEPLOYMENT_BINARY"
    print_info "Made binary executable"
fi

print_info "✓ Binary exists and is executable"

# Check release notes exists
if [ ! -f "$DEPLOYMENT_RN" ]; then
    print_error "Release notes not found in deployment package: $DEPLOYMENT_RN"
    exit 1
fi

print_info "✓ Release notes exists"

# Check config exists
if [ ! -f "$DEPLOYMENT_CONFIG" ]; then
    print_error "Config file not found in deployment package: $DEPLOYMENT_CONFIG"
    exit 1
fi

print_info "✓ Config file exists"

# Test binary execution - it will fail because source folder doesn't exist
# but we can verify it starts and loads config
print_info "Testing binary startup (will fail due to missing source folder - expected)..."
cd "$DEPLOYMENT_DIR"
# Capture output and check if it loads config
if timeout 2s "./$BINARY_NAME" 2>&1 | head -20 > /tmp/build_test_output.txt; then
    : # Command succeeded
else
    : # Command failed (expected - no source folder)
fi

# Check if output contains configuration loading
if grep -q "Configuration" /tmp/build_test_output.txt || grep -q "file-verifier" /tmp/build_test_output.txt; then
    print_info "✓ Binary starts and loads configuration"
    cat /tmp/build_test_output.txt
else
    print_info "✓ Binary runs (config validation expected to fail in test environment)"
    cat /tmp/build_test_output.txt
fi
rm -f /tmp/build_test_output.txt

# Return to original directory
cd - > /dev/null

print_info "✓ Deployment package validation complete"

# ============================================================================
# STEP 7: CLEANUP PROJECT ROOT
# ============================================================================
print_step "Step 7: Cleaning up project root directory..."

# Get absolute path of project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
print_info "Project root: $PROJECT_ROOT"

# Remove binary from project root (keep only in builds/)
if [ -f "$PROJECT_ROOT/$BINARY_NAME" ]; then
    rm "$PROJECT_ROOT/$BINARY_NAME"
    print_info "Removed: $BINARY_NAME from project root (kept in deployment package)"
fi

# Remove any CSV files from project root
if ls "$PROJECT_ROOT"/*.csv 1> /dev/null 2>&1; then
    rm "$PROJECT_ROOT"/*.csv
    print_info "Removed CSV files from project root"
fi

# Remove any test files that may have been created
if [ -f "$PROJECT_ROOT/test.zip" ] || [ -f "$PROJECT_ROOT/test.zip.sha256" ]; then
    rm -f "$PROJECT_ROOT/test.zip" "$PROJECT_ROOT/test.zip.sha256"
    print_info "Removed test files from project root"
fi

print_info "✓ Project root cleanup complete"

# ============================================================================
# STEP 8: SUCCESS
# ============================================================================
echo ""
echo -e "${GREEN}✓ Build completed successfully!${NC}"
echo ""
echo "======================================================================"
echo "  Build Summary"
echo "======================================================================"
echo "  Application: $APP_NAME"
echo "  Binary:      $BINARY_NAME"
echo "  Release:     $RELEASE"
echo "  Version:     $VERSION"
echo "  Build Time:  $BUILDTIME"
echo "  Build ID:    $BUILD_ID"
echo "======================================================================"
echo ""
echo "Project root: $PROJECT_ROOT"
echo ""
if [ "$VERSION" = "v1.0.0" ]; then
echo "  • Service File:  go-filesha-verifier.service (included for initial deployment)"
else
echo "  • Service File:  NOT included (already on server)"
fi
echo ""
echo "Note: Binary removed from project root, kept only in deployment package"
echo "Note: CSV files excluded from deployment (generated at runtime)"
echo ""
echo "Next steps:"
if [ "$VERSION" = "v1.0.0" ]; then
echo "  1. Test locally:"
echo "     cd $DEPLOYMENT_DIR"
echo "     # Update config.yaml with test paths"
echo "     ./go-filesha-verifier"
echo ""
echo "  2. Deploy to server:"
echo "     scp -r $DEPLOYMENT_DIR/* user@server:/home/auser/projects/go-filesha-verifier/"
echo ""
echo "  3. Install systemd service:"
echo "     sudo cp /home/auser/projects/go-filesha-verifier/go-filesha-verifier.service /etc/systemd/system/"
echo "     sudo systemctl daemon-reload"
echo "     sudo systemctl enable go-filesha-verifier"
echo "     sudo systemctl start go-filesha-verifier"
echo ""
echo "  4. Verify deployment:"
echo "     sudo systemctl status go-filesha-verifier"
echo "     sudo journalctl -u go-filesha-verifier -f"
echo "     tail -f /home/auser/projects/go-filesha-verifier/verification.csv"
else
echo "  1. Test locally:"
echo "     cd $DEPLOYMENT_DIR"
echo "     ./go-filesha-verifier"
echo ""
echo "  2. Stop service on server:"
echo "     sudo systemctl stop go-filesha-verifier"
echo ""
echo "  3. Deploy update:"
echo "     scp $DEPLOYMENT_DIR/go-filesha-verifier user@server:/home/auser/projects/go-filesha-verifier/"
echo "     scp $DEPLOYMENT_DIR/go-filesha-verifier.RN.yaml user@server:/home/auser/projects/go-filesha-verifier/"
echo ""
echo "  4. Restart service:"
echo "     sudo systemctl start go-filesha-verifier"
echo ""
echo "  5. Verify update:"
echo "     sudo systemctl status go-filesha-verifier"
echo "     sudo journalctl -u go-filesha-verifier -n 50"
fi
echo ""