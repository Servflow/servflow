#!/bin/bash
set -e

# Check required environment variables
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Error: GITHUB_TOKEN environment variable is required"
    exit 1
fi

if [ -z "$DRONE_TAG" ]; then
    echo "Error: DRONE_TAG environment variable is required"
    exit 1
fi

# Configuration
GITHUB_REPO="Servflow/servflow"
TAG_NAME="$DRONE_TAG"
RELEASE_NAME="Release $DRONE_TAG"
RELEASES_DIR="releases"

# Check if releases directory exists
if [ ! -d "$RELEASES_DIR" ]; then
    echo "Error: $RELEASES_DIR directory not found"
    exit 1
fi

# Create release body
RELEASE_BODY="Release $TAG_NAME

## Docker Images

Docker image: \`servflow/servflow:$TAG_NAME\`

## Installation

Download the appropriate binary for your platform from the assets below.

### Linux/macOS
\`\`\`bash
tar -xzf servflow-$TAG_NAME-linux-amd64.tar.gz
chmod +x servflow-*
sudo mv servflow-* /usr/local/bin/servflow
\`\`\`

### Windows
Extract the ZIP file and add the executable to your PATH.

## Checksums
Verify your download with the provided \`checksums.txt\` file:
\`\`\`bash
sha256sum -c checksums.txt
\`\`\`"

echo "Creating release for tag: $TAG_NAME"
echo "Repository: $GITHUB_REPO"
echo "Prerelease: $PRERELEASE"

# Create JSON payload
RELEASE_DATA=$(jq -n \
    --arg tag_name "$TAG_NAME" \
    --arg name "$RELEASE_NAME" \
    --arg body "$RELEASE_BODY" \
    --argjson prerelease "false" \
    '{
        tag_name: $tag_name,
        name: $name,
        body: $body,
        draft: false,
        prerelease: $prerelease
    }'
)

echo "Creating GitHub release..."

# Create the release
RELEASE_RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "Authorization: token $GITHUB_TOKEN" \
    -H "Content-Type: application/json" \
    -H "Accept: application/vnd.github.v3+json" \
    -d "$RELEASE_DATA" \
    "https://api.github.com/repos/$GITHUB_REPO/releases")

# Extract HTTP status code (last 3 characters)
HTTP_STATUS="${RELEASE_RESPONSE: -3}"
RELEASE_JSON="${RELEASE_RESPONSE%???}"

echo "HTTP Status: $HTTP_STATUS"

if [ "$HTTP_STATUS" != "201" ]; then
    echo "Failed to create release. Response:"
    echo "$RELEASE_JSON" | jq '.'
    exit 1
fi

echo "Release created successfully!"

# Extract upload URL
UPLOAD_URL=$(echo "$RELEASE_JSON" | jq -r '.upload_url' | sed 's/{?name,label}//')

if [ "$UPLOAD_URL" = "null" ] || [ -z "$UPLOAD_URL" ]; then
    echo "Failed to get upload URL from release response"
    echo "$RELEASE_JSON" | jq '.'
    exit 1
fi

echo "Upload URL: $UPLOAD_URL"

# Upload assets
echo "Uploading release assets..."

UPLOAD_COUNT=0
for file in "$RELEASES_DIR"/*; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        echo "Uploading $filename..."

        UPLOAD_RESPONSE=$(curl -s -w "%{http_code}" -X POST \
            -H "Authorization: token $GITHUB_TOKEN" \
            -H "Content-Type: application/octet-stream" \
            -H "Accept: application/vnd.github.v3+json" \
            --data-binary @"$file" \
            "$UPLOAD_URL?name=$filename")

        UPLOAD_STATUS="${UPLOAD_RESPONSE: -3}"
        UPLOAD_JSON="${UPLOAD_RESPONSE%???}"

        if [ "$UPLOAD_STATUS" = "201" ]; then
            echo "✓ Successfully uploaded $filename"
            UPLOAD_COUNT=$((UPLOAD_COUNT + 1))
        else
            echo "✗ Failed to upload $filename (HTTP $UPLOAD_STATUS)"
            echo "$UPLOAD_JSON" | jq '.' || echo "$UPLOAD_JSON"
        fi
    fi
done

echo ""
echo "Upload Summary:"
echo "- Files uploaded: $UPLOAD_COUNT"
echo "- Release URL: https://github.com/$GITHUB_REPO/releases/tag/$TAG_NAME"
echo ""
echo "Release created successfully! 🎉"
