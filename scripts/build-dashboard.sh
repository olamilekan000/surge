#!/bin/bash
set -e

echo "Building Dashboard..."

cd "$(dirname "$0")/../dashboard/surge-dashboard"

if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

echo "Building dashboard..."
npm run build

DEST_DIR="../../surge/server/dashboard-dist"
echo "Copying build files to $DEST_DIR..."

rm -rf "$DEST_DIR"/*

cp -r dist/* "$DEST_DIR"

echo "Dashboard built and copied to $DEST_DIR"
echo "You can now build the Go application and the dashboard will be embedded"
