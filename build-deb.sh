#!/bin/bash

# Exit on error
set -e

# Variables
APP_NAME="asynchook"       # Name of your application
VERSION="1.0.2"            # Version of your application
ARCH="amd64"               # Architecture (amd64, arm64, all, etc.)
BUILD_DIR="./build"        # Temporary build directory
PACKAGE_DIR="./package"    # Package structure directory
DEB_FILE="./${APP_NAME}_${VERSION}_${ARCH}.deb"  # Output .deb file

# Step 1: Build the Go application
echo "Building Go application..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"
GOOS=linux GOARCH=$ARCH go build -o "$BUILD_DIR/$APP_NAME" .

# Step 2: Set up the package structure
echo "Setting up package structure..."
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR/DEBIAN"
mkdir -p "$PACKAGE_DIR/usr/bin"
mkdir -p "$PACKAGE_DIR/lib/systemd/system"

# Step 3: Create control file
echo "Creating control file..."
cat <<EOF > "$PACKAGE_DIR/DEBIAN/control"
Package: $APP_NAME
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: Jay padaliya <developer.padaliyajay@gmail.com>
Description: $APP_NAME
EOF

# Step 4: Copy application binary and service file
echo "Copying application binary..."
cp "$BUILD_DIR/$APP_NAME" "$PACKAGE_DIR/usr/bin/"
echo "Copying service file..."
cat <<EOF > "$PACKAGE_DIR/lib/systemd/system/$APP_NAME.service"
[Unit]
Description=$APP_NAME Service
After=network.target

[Service]
ExecStart=/usr/bin/$APP_NAME --config=/etc/$APP_NAME/config.yaml
Restart=on-failure
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF

# Step 5: Build the .deb package
echo "Building .deb package..."
dpkg-deb --build "$PACKAGE_DIR" "$DEB_FILE"

# Step 6: Clean up
echo "Cleaning up..."
rm -rf "$BUILD_DIR"
rm -rf "$PACKAGE_DIR"

# Step 7: Output result
echo "Debian package created successfully: $DEB_FILE"
