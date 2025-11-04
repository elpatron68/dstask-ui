# Release Process

This document describes the release process and changelog conventions for dstask-web.

## Versioning
- SemVer (MAJOR.MINOR.PATCH)
- Example: `v0.1.6`

## Changelog conventions
- Commit message prefixes:
  - `feat:` new features
  - `fix:` bug fixes
  - `docs:` documentation
  - `test:` tests
  - `ci:` CI/CD
  - `refactor:` internal refactors with no behavior change
  - `perf:`, `build:`, `chore:` optional
- Release notes are generated automatically by GitHub from commits (generate_release_notes).

## How to cut a release
1. Ensure CI is green (build + tests + staticcheck).
2. Create and push a tag:
   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
3. The CI workflow (`CI`) builds cross-platform artifacts and creates a GitHub Release for version tags (with auto release notes).
   - If the tag was created before workflows existed: run the manual workflow "Create Release (manual)" in GitHub Actions to generate a release retroactively.

## Coverage (optional)
- If `CODECOV_TOKEN` is set as a repository secret, the CI job uploads coverage to Codecov.
- You can switch the README badge to the Codecov badge if desired.

## Artifacts
- CI cross-compiles binaries for Linux, macOS, Windows (amd64/arm64) and uploads them to the GitHub Release.

## Hotfixes
- Branch from `master`, apply fixes, ensure CI passes, then tag and push a new PATCH release.


