#!/bin/sh

# Install git hooks for ServFlow

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

echo "Installing git hooks..."

# Check if .git directory exists
if [ ! -d "$PROJECT_ROOT/.git" ]; then
    echo "Error: .git directory not found. Are you in a git repository?"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Install pre-push hook
cp "$SCRIPT_DIR/pre-push" "$HOOKS_DIR/pre-push"
chmod +x "$HOOKS_DIR/pre-push"

echo "✓ Installed pre-push hook"
echo "Git hooks installed successfully!"
