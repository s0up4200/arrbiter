#!/bin/bash
# Claude Code auto-formatting hook script
# Formats files with gofmt (for .go) or prettier (for everything else)

# Log to a debug file
DEBUG_LOG="/Users/soup/github/autobrr/netronome/.claude/format-debug.log"
echo "[$(date)] Hook triggered" >>"$DEBUG_LOG"

# Read JSON input from stdin
JSON_INPUT=$(cat)
echo "[$(date)] JSON input: $JSON_INPUT" >>"$DEBUG_LOG"

# Extract file path from JSON using jq or sed
# First try with jq if available
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(echo "$JSON_INPUT" | jq -r '.tool_input.file_path // empty')
    echo "[$(date)] File path (jq): $FILE_PATH" >>"$DEBUG_LOG"
else
    # Fallback to sed/grep
    FILE_PATH=$(echo "$JSON_INPUT" | grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    echo "[$(date)] File path (sed): $FILE_PATH" >>"$DEBUG_LOG"
fi

# Exit if no file path provided
if [ -z "$FILE_PATH" ]; then
    echo "No file path provided" >&2
    exit 0
fi

# Exit if file doesn't exist
if [ ! -f "$FILE_PATH" ]; then
    echo "File does not exist: $FILE_PATH" >&2
    exit 0
fi

# Skip vendor directory
if [[ "$FILE_PATH" == *"/vendor/"* ]]; then
    echo "Skipping vendor file: $FILE_PATH" >&2
    exit 0
fi

# Get file extension
EXTENSION="${FILE_PATH##*.}"
FILENAME=$(basename "$FILE_PATH")

# Format based on file type
if [ "$EXTENSION" = "go" ]; then
    # Check if gofmt is available
    if command -v gofmt >/dev/null 2>&1; then
        echo "Formatting Go file with gofmt: $FILE_PATH" >&2
        gofmt -w "$FILE_PATH"
    else
        echo "gofmt not found, skipping Go formatting" >&2
    fi
else
    # Check if prettier is available
    if command -v prettier >/dev/null 2>&1; then
        echo "Formatting file with prettier: $FILE_PATH" >&2
        # Use --ignore-unknown to skip files prettier doesn't support
        prettier --write --ignore-unknown "$FILE_PATH" 2>/dev/null || {
            echo "Prettier formatting failed or file type not supported" >&2
        }
    else
        echo "prettier not found, skipping formatting" >&2
    fi
fi

# Always exit successfully to avoid blocking Claude Code
exit 0
