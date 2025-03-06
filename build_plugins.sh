#!/bin/bash
set -e

PLUGIN_SRC_DIR="plugins_src"
PLUGIN_BUILD_DIR="plugins"

# Create the build directory if it doesn't exist.
mkdir -p "$PLUGIN_BUILD_DIR"

# Iterate over each .go file in the plugins_src directory.
for file in "$PLUGIN_SRC_DIR"/*.go; do
    base=$(basename "$file" .go)
    
    # Extract the plugin version from the file by looking for Version() method
    version=$(grep -A 1 "func.*Version()" "$file" | grep "return" | grep -o '"[^"]*"' | tr -d '"')
    
    # If version is not found, use a timestamp
    if [ -z "$version" ]; then
        version=$(date +%s)
    fi
    
    # Create a unique filename using the version
    versioned_filename="${base}_v${version}.so"
    
    echo "Building plugin: $base (version $version)"
    
    # Build the plugin using the unique build tag.
    go build -tags "$base" -buildmode=plugin -o "$PLUGIN_BUILD_DIR/$versioned_filename" "$file"
    
    # Create a symlink to the latest version for easier loading
    ln -sf "$versioned_filename" "$PLUGIN_BUILD_DIR/$base.so"
done

echo "All plugins have been built and moved to the '$PLUGIN_BUILD_DIR' folder."