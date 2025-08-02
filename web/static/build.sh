#!/bin/bash

# Build script for OpenTrail frontend
set -e

echo "Building OpenTrail frontend..."

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# Build the project
echo "Building React application..."
npm run build

# Copy built files to replace the old static files
echo "Copying built files..."
cp dist/index.html ./
cp dist/style.css ./
cp dist/app.js ./

# Clean up dist directory
rm -rf dist

echo "Build completed successfully!"
echo "Files updated:"
echo "  - index.html"
echo "  - style.css" 
echo "  - app.js"