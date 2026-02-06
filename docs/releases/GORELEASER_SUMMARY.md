# GoReleaser Support - Summary

## Overview

Added full GoReleaser support for automated releases with deb/rpm packages targeting Linux x86_64.

## What Was Added

### 1. GoReleaser Configuration (`.goreleaser.yaml`)
- **Build**: Linux x86_64 binary with ldflags for version info
- **Archives**: tar.gz with docs and config files
- **Packages**: 
  - Debian/Ubuntu (.deb)
  - RHEL/CentOS/Fedora (.rpm)
- **Package Contents**:
  - Binary installed to `/usr/local/bin/lldiscovery`
  - Config file at `/etc/lldiscovery/config.json` (config|noreplace)
  - Systemd service at `/etc/systemd/system/lldiscovery.service`
  - Data directory at `/var/lib/lldiscovery`
- **Changelog**: Auto-generated from git commits
- **Release Notes**: Include installation instructions

### 2. Package Scripts (`scripts/`)
- **preinstall.sh**: Creates system user and directories
- **postinstall.sh**: Sets permissions, reloads systemd, prints usage instructions
- **preremove.sh**: Stops and disables service before removal

### 3. GitHub Actions (`.github/workflows/release.yml`)
- Triggers on tags starting with `v` (e.g., `v1.0.0`)
- Uses Go 1.25.6
- Runs tests before building
- Publishes release to GitHub automatically

### 4. Version Information
- Extended `main.go` with `commit` and `date` variables
- Enhanced `--version` output to show:
  ```
  lldiscovery 1.0.0
    commit: abc1234
    built:  2026-02-05T02:00:00Z
  ```

### 5. Documentation
- **README.md**: Added installation section with package instructions
- **RELEASING.md**: Complete release process guide for maintainers
- **CHANGELOG.md**: Added GoReleaser entry
- **LICENSE**: MIT license file

### 6. Configuration
- **.gitignore**: Added dist/, *.deb, *.rpm, *.tar.gz

## Package Features

### Automatic Setup
Packages handle complete setup:
1. Create `lldiscovery` system user
2. Create `/etc/lldiscovery/` and `/var/lib/lldiscovery/`
3. Install and configure systemd service
4. Set proper ownership (user:group)
5. Set proper permissions (config: 0640)

### User Instructions
After installation, package prints:
```
lldiscovery has been installed.

To enable and start the service:
  sudo systemctl enable lldiscovery
  sudo systemctl start lldiscovery

To check status:
  sudo systemctl status lldiscovery

Configuration file: /etc/lldiscovery/config.json
Logs: journalctl -u lldiscovery -f
```

### Clean Removal
On package removal:
1. Stops service if running
2. Disables service if enabled
3. Removes binary and systemd service file
4. Preserves config and data (per package manager standards)

## Testing

Tested locally:
```bash
# Validate configuration
goreleaser check
✓ 1 configuration file(s) validated

# Test build
goreleaser build --snapshot --clean
✓ build succeeded

# Verify binary
./dist/lldiscovery_linux_amd64_v1/lldiscovery --version
lldiscovery 0.0.0-next
  commit: none
  built:  2026-02-05T00:00:48Z

# All integration tests pass
./test.sh
==> All tests passed! ✓
```

## Release Process

To create a release:
```bash
# 1. Update CHANGELOG.md
# 2. Commit changes
git add CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main

# 3. Create and push tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

GitHub Actions will automatically:
- Build binaries
- Create packages
- Generate release notes
- Publish to GitHub Releases

## Installation Examples

### Debian/Ubuntu
```bash
wget https://github.com/kad/lldiscovery/releases/download/v1.0.0/lldiscovery_1.0.0_linux_amd64.deb
sudo dpkg -i lldiscovery_1.0.0_linux_amd64.deb
sudo systemctl enable --now lldiscovery
```

### RHEL/CentOS/Fedora
```bash
wget https://github.com/kad/lldiscovery/releases/download/v1.0.0/lldiscovery_1.0.0_linux_amd64.rpm
sudo rpm -i lldiscovery_1.0.0_linux_amd64.rpm
sudo systemctl enable --now lldiscovery
```

### Binary
```bash
wget https://github.com/kad/lldiscovery/releases/download/v1.0.0/lldiscovery_1.0.0_linux_amd64.tar.gz
tar xzf lldiscovery_1.0.0_linux_amd64.tar.gz
sudo cp lldiscovery /usr/local/bin/
# Manual setup required (see README.md)
```

## Files Changed/Created

### New Files
- `.goreleaser.yaml` - GoReleaser configuration
- `.github/workflows/release.yml` - GitHub Actions workflow
- `scripts/preinstall.sh` - Pre-installation script
- `scripts/postinstall.sh` - Post-installation script
- `scripts/preremove.sh` - Pre-removal script
- `LICENSE` - MIT license
- `RELEASING.md` - Release process documentation

### Modified Files
- `cmd/lldiscovery/main.go` - Added commit/date version fields
- `README.md` - Added installation section
- `CHANGELOG.md` - Added GoReleaser entry
- `.gitignore` - Added dist/ and package files

## Benefits

1. **Professional Releases**: Automated, consistent releases
2. **Easy Installation**: Native packages for major Linux distros
3. **Complete Setup**: Packages handle all setup automatically
4. **Version Tracking**: Full version, commit, and build date info
5. **Maintainability**: Clear release process with documentation
6. **Security**: Proper user/group isolation and file permissions

## Future Enhancements

Consider adding:
- ARM64 support (add `arm64` to goarch in `.goreleaser.yaml`)
- Package signing (GPG keys)
- Homebrew formula
- Docker images
- Checksums verification in README
