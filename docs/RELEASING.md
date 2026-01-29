# Releasing tpg

This document describes the release process for tpg.

## Prerequisites

- `gh` CLI installed and authenticated: `brew install gh && gh auth login`
- Push access to the repository

## Version Scheme

tpg uses [Semantic Versioning](https://semver.org/):
- **patch** (0.0.x): Bug fixes, minor improvements
- **minor** (0.x.0): New features, non-breaking changes  
- **major** (x.0.0): Breaking changes

## Release Process

### 1. Check changes since last release

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD
```

### 2. Create annotated tag

Annotated tags are required for GitHub releases. Include release notes in the tag message:

```bash
git tag -a v0.2.1 -m "v0.2.1

## Changes

- Fixed bug X
- Added feature Y
- Improved Z

### Breaking Changes (if any)

- Removed deprecated flag --foo
"
```

### 3. Push the tag

```bash
git push origin v0.2.1
```

### 4. Create GitHub release

```bash
gh release create v0.2.1 --title "v0.2.1" --notes-from-tag
```

This creates a release using the annotated tag's message as release notes.

## Using the Slash Command

In OpenCode, you can use the `/release` command to automate this process:

```
/release patch    # Bump patch version (0.0.x)
/release minor    # Bump minor version (0.x.0)  
/release major    # Bump major version (x.0.0)
/release v1.2.3   # Use specific version
```

## Installing a Release

Users can install any released version:

```bash
go install github.com/taxilian/tpg/cmd/tpg@v0.2.1
go install github.com/taxilian/tpg/cmd/tpg@latest
```

## Troubleshooting

### Tag already exists

If you need to redo a tag:

```bash
# Delete locally and remotely
git tag -d v0.2.1
git push origin :refs/tags/v0.2.1

# Recreate
git tag -a v0.2.1 -m "..."
git push origin v0.2.1
```

### Release not showing on GitHub

Pushing a tag alone doesn't create a GitHub Release. You must either:
1. Use `gh release create`
2. Create via GitHub web UI
3. Use GitHub Actions with goreleaser
