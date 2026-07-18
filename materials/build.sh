#!/bin/bash

# Build the materials CLI binary.
# Run from inside the materials/ directory.

cd "$(dirname "$0")/.." || exit 1
go build -o materials/Build_Material ./materials/
echo "Built: materials/Build_Material"
