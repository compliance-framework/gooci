# Goreleaser to OCI

In the compliance framework, we distribute plugins as OCI artifacts for use in the collection agent.

In order to efficiently distribute the plugins for multiple operating systems and architectures, we took
a page from the Homebrew playbook, and decided to upload our binaries and files to OCI, so we could easily
version and distribute them for all sorts of runtimes.

`gooci` is a CLI we use to take goreleaser builds and archives and upload them to an OCI registry.

When the plugins are then used it's easy for us to just specify the OCI path
`ghcr.io/compliance-framework/plugin-ubuntu-vulnerabilities:1.0.1`, and know it will work no matter
where it runs.

## Installation

Using `go install`
```shell
go install github.com/compliance-framework/gooci@0.0.2
```

From source
```shell
# Build from source
git clone git@github.com:compliance-framework/goreleaser-oci.git
cd goreleaser-oci
go mod download
go build -o gooci main.go
sudo cp gooci /usr/local/bin
```

## Usage

```shell
$ gooci help
gooci handles uploading and downloading of GoReleaser archives to an OCI registry

Usage:
  gooci [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  download    Download GoReleaser archives to a local directory
  help        Help about any command
  login       Login to a remote registry
  logout      Log out of a registry
  upload      Upload GoReleaser archives to an OCI registry

Flags:
  -h, --help   help for gooci

Use "gooci [command] --help" for more information about a command.
```

## Releasing

Releases use the shared [compliance-framework/workflows](https://github.com/compliance-framework/workflows)
release flow (`release-please.yml`, `release-go-lib.yml`, `release-checks.yml`):

1. Merge conventional-commit PRs to `main`; `release-please` opens or updates the release PR
   (`release-please--branches--main`), versioned from `.release-please-manifest.json`.
2. Merging the release PR tags `vX.Y.Z` and publishes the GitHub release.
3. `release` (on `release: published`) runs goreleaser on the tag and attaches the archives
   to that release (`.goreleaser.yaml` keeps `release.prerelease: auto`).

Release tags are created by the release bot only; don't push `v*` tags by hand. To test a
release candidate, run the `cut-prerelease` workflow: it tags `vX.Y.Z-rcN` from the open
release PR's version and publishes a pre-release, which `release` builds the same way.
