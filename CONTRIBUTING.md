# Contributing to no-pilot

## Workflow

- **Trunk-based development:** All work is merged to `main`. Feature and fix branches should be short-lived.
- **Pull Requests:** Open a PR for all changes. All PRs must pass CI before merging.
- **Conventional Commits:** Use [Conventional Commits](https://www.conventionalcommits.org/) for all commit messages **and PR titles** (e.g., `feat:`, `fix:`, `chore:`). This is critical — GitHub uses the PR title as the squash commit message, so a non-conforming PR title (e.g. `Fix something` instead of `fix: something`) will be invisible to release-please and won't trigger a version bump.
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

## Local Development (devcontainer)

The devcontainer is the recommended development environment. The workspace `.vscode/mcp.json` launches the server with `go run .` so Copilot always runs the current source without requiring a separate build step.

**Why `go run .` instead of a pre-built binary?**
VS Code starts the MCP server immediately on window attach, before any binary has been built. Pointing at a binary path causes `ENOENT` if the binary is not yet on disk. `go run .` compiles on demand using the Go build cache and is always ready.

After making changes to the server code, restart the MCP server to recompile:

**Command Palette → MCP: Restart Server → no-pilot**

To build and install the binary explicitly (e.g. for testing the installed binary or cutting a release build):

```sh
make install
```

> [!NOTE]
> `make install` is **not** run automatically on every VS Code attach. The MCP server uses `go run .` directly, so the installed binary is only needed if you are explicitly testing it.

## Example Commit Messages

| Prefix | Version Bump | When to use |
|---|---|---|
| `feat:` | minor (1.**1**.0) | New user-facing feature |
| `fix:` | patch (1.0.**1**) | Bug fix |
| `chore:`, `docs:`, `refactor:`, `test:` | none | No release triggered |
| `feat!:` or `fix!:` | major (**2**.0.0) | Breaking change |

- `feat: add support for new tool`
- `fix: correct policy enforcement bug`
- `chore: update dependencies`
- `feat!: redesign policy config format` _(breaking change — bumps major)_

## Questions?

Open an issue or discussion on GitHub!
