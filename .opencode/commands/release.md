---
description: Create a new release with version bump and GitHub release
---

# Release Process

Create a new release for tpg. This will:
1. Determine the next version number based on changes
2. Create an annotated git tag
3. Push the tag to GitHub
4. Create a GitHub release with release notes

## Steps

1. First, check what has changed since the last release:
   ```bash
   git log --oneline $(git describe --tags --abbrev=0)..HEAD
   ```

2. Determine the version bump:
   - **patch** (0.0.x): Bug fixes, minor improvements
   - **minor** (0.x.0): New features, non-breaking changes
   - **major** (x.0.0): Breaking changes

3. Get the current version and calculate the new version:
   ```bash
   git describe --tags --abbrev=0
   ```

4. Create an annotated tag with release notes summarizing the changes:
   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z

   ## Changes

   - Change 1
   - Change 2
   "
   ```

5. Push the tag:
   ```bash
   git push origin vX.Y.Z
   ```

6. Create the GitHub release:
   ```bash
   gh release create vX.Y.Z --title "vX.Y.Z" --notes-from-tag
   ```

## Arguments

If an argument is provided, use it as the version type (patch, minor, major) or exact version (vX.Y.Z).

Execute this release process now. Ask for confirmation before creating the tag.
