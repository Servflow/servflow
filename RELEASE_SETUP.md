# Release Setup Guide

This document explains how to set up the automated release workflow for the Servflow project, where binaries are built in a private repository and released to this public repository.

## Overview

The release process follows this flow:
1. **Private Repository**: Contains source code and build workflows
2. **Public Repository**: Serves as distribution point for binaries and issue tracking
3. **Automated Release**: GitHub Actions build binaries and push releases to the public repo

## Setup Instructions

### 1. Private Repository Setup

#### A. Copy the Release Workflow

Copy the example workflow from this repository to your private repo:

```bash
# In your private repository
mkdir -p .github/workflows
cp /path/to/servflow-release/.github/workflows/release-example.yml .github/workflows/release.yml
```

#### B. Customize the Workflow

Edit `.github/workflows/release.yml` and update:

- `PUBLIC_REPO`: Change to your actual public repository name (e.g., `username/servflow`)
- `BINARY_NAME`: Ensure it matches your binary name
- Build configuration: Adjust Go version, build flags, and platforms as needed

#### C. Add Installation Script

Copy the installation script to your private repo:

```bash
cp /path/to/servflow-release/install.sh ./install.sh
```

### 2. GitHub Token Setup

#### A. Create Personal Access Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Set expiration and select these scopes:
   - `repo` (Full control of private repositories)
   - `public_repo` (Access to public repositories)
   - `write:packages` (Write packages to GitHub Package Registry)

#### B. Add Token to Private Repository

1. Go to your private repository
2. Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Name: `PUBLIC_REPO_TOKEN`
5. Value: Your personal access token

### 3. Repository Configuration

#### A. Public Repository Settings

1. **Enable Issues**: Settings → General → Features → Issues (checked)
2. **Enable Discussions** (optional): Settings → General → Features → Discussions (checked)
3. **Release permissions**: Settings → Actions → General → Fork pull request workflows from outside collaborators

#### B. Package Registry Setup

1. Go to your public repository
2. Settings → Pages (if you want GitHub Pages for docs)
3. Settings → Security → Secrets and variables → Actions
4. Ensure `GITHUB_TOKEN` has package write permissions

### 4. Release Process

#### Creating a Release

1. **In your private repository**, create and push a version tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically**:
   - Build binaries for all platforms
   - Create Docker images
   - Generate release notes
   - Publish to the public repository

#### Version Tagging Convention

Use semantic versioning:
- `v1.0.0` - Major release
- `v1.0.1` - Patch release
- `v1.1.0` - Minor release
- `v2.0.0-beta.1` - Pre-release (marked as prerelease)
- `v2.0.0-rc.1` - Release candidate (marked as prerelease)

### 5. Customization Options

#### Build Targets

Modify the matrix in `release.yml` to add/remove build targets:

```yaml
strategy:
  matrix:
    include:
      - goos: linux
        goarch: amd64
        suffix: linux-amd64
      # Add more platforms as needed
```

#### Version Information

The workflow injects version information into the binary:

```go
// In your main.go or version.go
var (
    version = "dev"
    commit  = "unknown"
    date    = "unknown"
)
```

#### Docker Configuration

Ensure your `Dockerfile` supports multi-platform builds:

```dockerfile
# Use build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Set labels
LABEL org.opencontainers.image.version="$VERSION"
LABEL org.opencontainers.image.revision="$COMMIT"
LABEL org.opencontainers.image.created="$DATE"
```

### 6. Testing the Setup

#### Dry Run Test

1. Create a test tag:
   ```bash
   git tag v0.0.1-test
   git push origin v0.0.1-test
   ```

2. Check GitHub Actions in your private repo
3. Verify release appears in your public repo
4. Test binary downloads and installation

#### Cleanup Test Release

```bash
# Delete remote tag
git push origin --delete v0.0.1-test

# Delete local tag
git tag -d v0.0.1-test

# Delete GitHub release (via web interface or API)
```

### 7. Monitoring and Maintenance

#### Regular Tasks

- **Update dependencies**: Keep GitHub Actions updated
- **Monitor releases**: Check that all platforms build successfully
- **Update documentation**: Keep README and installation instructions current

#### Troubleshooting

**Build Failures:**
- Check GitHub Actions logs in private repository
- Verify all required secrets are set
- Ensure token has necessary permissions

**Release Not Created:**
- Verify `PUBLIC_REPO_TOKEN` is valid and has correct permissions
- Check that public repository name is correct in workflow
- Ensure tag follows expected pattern (`v*`)

**Docker Issues:**
- Verify container registry permissions
- Check Dockerfile syntax and build context
- Ensure multi-platform build support

### 8. Security Considerations

- **Token Security**: Regularly rotate personal access tokens
- **Scope Limitation**: Use tokens with minimal required permissions
- **Repository Access**: Limit who can push tags in private repository
- **Audit Releases**: Monitor release creation and binary uploads

### 9. Alternative Setups

#### Using GitHub App

Instead of personal access tokens, you can use a GitHub App:

1. Create a GitHub App with repository permissions
2. Install the app on both repositories
3. Use the app's JWT token in workflows

#### Multi-Repository Organizations

For organizations with multiple repositories:

1. Use organization secrets for shared tokens
2. Create reusable workflows
3. Implement approval processes for releases

## Support

If you encounter issues with the release setup:

1. Check the [GitHub Actions documentation](https://docs.github.com/en/actions)
2. Review workflow logs for error messages
3. Create an issue in this repository with setup questions
4. Contact the development team for private repository access issues