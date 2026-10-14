package oci

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TestDownloadPublishedArtifacts runs Downloader.Download against artifacts already
// published to ghcr.io, the ones the stack smoke test uses. Each is pinned by digest, so the
// test does not change when a tag moves.
//
// It needs network access to ghcr.io and runs by default (CI included). Skip it offline with
// `go test -short ./...` or GOOCI_SKIP_NETWORK_TESTS=1. It pulls anonymously: DOCKER_CONFIG
// points at an empty directory, so local registry credentials are not used.
func TestDownloadPublishedArtifacts(t *testing.T) {
	if testing.Short() || os.Getenv("GOOCI_SKIP_NETWORK_TESTS") == "1" {
		t.Skip("network test: skipped by -short or GOOCI_SKIP_NETWORK_TESTS=1")
	}
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	tests := []struct {
		name string
		// ref is an image index digest.
		ref string
		// want lists every regular file the artifact's layer holds.
		want []string
		// executable files must keep an execute bit.
		executable []string
	}{
		{
			// v0.2.0: an `opa build` bundle; every tar entry starts with "/".
			name: "policy bundle",
			ref:  "ghcr.io/compliance-framework/plugin-file-attestation-policies@sha256:ccf904e3d9ba88df8a141a013edbf96c0541f5a692f4285adabbe5c746374f33",
			want: []string{
				".manifest",
				"data.json",
				"policies/critical_file_attestation_digest.rego",
				"policies/critical_file_attestation_digest_test.rego",
				"policies/critical_file_attestation_signer.rego",
				"policies/critical_file_attestation_signer_test.rego",
				"policies/critical_file_exists.rego",
				"policies/critical_file_exists_test.rego",
			},
		},
		{
			// v0.2.0: a goreleaser archive (linux/amd64 picked from the index); entries
			// have no leading "/".
			name:       "plugin",
			ref:        "ghcr.io/compliance-framework/plugin-file-attestation@sha256:63d51a91a1be8bd2b45f8910ff36c47f80a2f8ae9043cc8ca754afcff7356893",
			want:       []string{"LICENSE", "README.md", "plugin"},
			executable: []string{"plugin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dest := newDestination(t)
			ref, err := name.ParseReference(tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			dl := Downloader{destination: dest, reference: ref}

			err = dl.Download(remote.WithPlatform(v1.Platform{OS: "linux", Architecture: "amd64"}))
			if err != nil {
				t.Fatalf("download %s: %v", tt.ref, err)
			}

			files := extractedFiles(t, dest)
			assertFileSet(t, files, tt.want)
			for _, rel := range tt.executable {
				if mode, ok := files[rel]; ok && mode&0o111 == 0 {
					t.Errorf("%s mode = %v, want it executable", rel, mode)
				}
			}
			if outside := filesOutside(t, root, dest); len(outside) > 0 {
				t.Errorf("files were written outside the destination: %v", outside)
			}
		})
	}
}
