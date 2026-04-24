# Contributing to no-pilot

## Workflow

- **Trunk-based development:** All work is merged to `main`. Feature and fix branches should be short-lived.
- **Pull Requests:** Open a PR for all changes. All PRs must pass CI before merging.
- **Conventional Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/) for all commit messages (e.g., `feat:`, `fix:`, `chore:`). This enables automated changelogs and versioning.
- **Code Review:** All PRs require review and must pass all tests before merge.

## Release Process

- **Automated Releases:**
  - Releases are managed by [release-please](https://github.com/googleapis/release-please).
  - When you want a release, let release-please open a PR with a version bump and changelog.
  - Merging the release-please PR to `main` creates a GitHub Release and tag.
  - The release build workflow is triggered, building and uploading binaries for all major platforms.
- **Never tag manually.** Let release-please handle versioning and changelogs.

## CI/CD

- **Tests:**
  - All PRs and pushes to `main` run the full test suite in CI.
  - Merges are blocked if tests fail.
- **Release Builds:**
  - On every tagged release, binaries for Linux, macOS, and Windows (amd64/arm64) are built and attached to the GitHub Release automatically.

## How to Contribute

1. Create a feature or fix branch from `main`.
2. Make your changes, following Go best practices and using Conventional Commits.
3. Run tests locally: `go test ./...`
4. Open a PR against `main`.
5. Ensure all checks pass and request a review.
6. For releases, let release-please handle the process.

## Example Commit Messages

- `feat: add support for new tool`
- `fix: correct policy enforcement bug`
- `chore: update dependencies`

## Questions?

Open an issue or discussion on GitHub!
