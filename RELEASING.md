# Release Process

This document describes how to create a new release of lldiscovery.

## Prerequisites

- Push access to the GitHub repository
- Git configured with signing key (optional but recommended)

## Release Steps

### 1. Update Version

Update the CHANGELOG.md:
- Move items from `[Unreleased]` to a new version section
- Follow [Semantic Versioning](https://semver.org/):
  - MAJOR: Breaking changes
  - MINOR: New features (backward compatible)
  - PATCH: Bug fixes

Example:
```markdown
## [1.2.0] - 2026-02-05

### Added
- New feature X

### Changed
- Improved feature Y

### Fixed
- Bug Z
```

### 2. Commit Changes

```bash
git add CHANGELOG.md
git commit -m "chore: prepare release v1.2.0"
git push origin main
```

### 3. Create and Push Tag

```bash
# Create annotated tag
git tag -a v1.2.0 -m "Release v1.2.0"

# Push tag to trigger release workflow
git push origin v1.2.0
```

### 4. GitHub Actions Automation

When you push a tag starting with `v`:
1. GitHub Actions workflow `.github/workflows/release.yml` triggers
2. GoReleaser runs and:
   - Builds binaries for Linux x86_64
   - Creates .deb and .rpm packages
   - Generates changelog from commits
   - Creates checksums
   - Creates GitHub Release with all artifacts
3. Release is published automatically

### 5. Verify Release

Check the [Releases page](https://github.com/akanevsk/lldiscovery/releases) to ensure:
- Binary archives are available
- .deb package is available
- .rpm package is available
- Checksums file is present
- Release notes are correct

## Local Testing

Test the release process locally without publishing:

```bash
# Test build
goreleaser build --snapshot --clean

# Test full release (no publish)
goreleaser release --snapshot --clean
```

Artifacts will be in the `dist/` directory.

## Manual Release (Emergency)

If GitHub Actions fails, you can release manually:

```bash
# Ensure you're on the correct tag
git checkout v1.2.0

# Set GITHUB_TOKEN
export GITHUB_TOKEN="your_github_token"

# Run goreleaser
goreleaser release --clean
```

## Package Signing (Future)

Currently packages are not signed. To add signing:
1. Generate GPG key
2. Add signing configuration to `.goreleaser.yaml`
3. Store GPG key in GitHub Secrets
4. Update workflow to import key

## Troubleshooting

**Build fails:**
- Check `go.mod` and `go.sum` are up to date
- Run `go test ./...` locally
- Check GoReleaser config: `goreleaser check`

**Release not created:**
- Ensure tag starts with `v` (e.g., `v1.0.0`)
- Check GitHub Actions logs
- Verify `GITHUB_TOKEN` has correct permissions

**Packages fail to install:**
- Test packages locally:
  ```bash
  # For .deb
  dpkg-deb --info dist/lldiscovery_*.deb
  
  # For .rpm
  rpm -qip dist/lldiscovery_*.rpm
  ```
