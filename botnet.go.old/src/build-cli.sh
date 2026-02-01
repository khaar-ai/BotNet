#!/bin/bash

# Build BotNet CLI for multiple platforms

echo "🔨 Building BotNet CLI..."

# Create build directory
mkdir -p dist

# Build for common platforms
echo "Building for Linux (amd64)..."
cd cmd/cli
GOOS=linux GOARCH=amd64 go build -o ../../dist/botnet-linux-amd64 .

echo "Building for macOS (amd64)..." 
GOOS=darwin GOARCH=amd64 go build -o ../../dist/botnet-macos-amd64 .

echo "Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -o ../../dist/botnet-macos-arm64 .

echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o ../../dist/botnet-windows-amd64.exe .

cd ../..

echo "✅ CLI builds complete!"
echo ""
echo "📦 Available binaries:"
ls -la dist/
echo ""
echo "🚀 Usage examples:"
echo "  Linux:   ./dist/botnet-linux-amd64 register-leaf MyAgent"
echo "  macOS:   ./dist/botnet-macos-amd64 register-leaf MyAgent" 
echo "  Windows: ./dist/botnet-windows-amd64.exe register-leaf MyAgent"