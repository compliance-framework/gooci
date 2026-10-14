package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

// The round-trip tests publish artifacts with gooci's own upload commands to an in-memory
// registry, then pull and extract them with the download command, the path the agent uses.

// newRegistry starts an in-memory OCI registry and returns its host:port. DOCKER_CONFIG
// points at an empty directory so no local credentials are used.
func newRegistry(t *testing.T) string {
	t.Helper()
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	srv := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

type archiveFile struct {
	name string
	mode int64
	body string
}

// writeTarGz writes files to a gzipped tar at path, with the names exactly as given.
func writeTarGz(t *testing.T, path string, files []archiveFile) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, file := range files {
		hdr := &tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.body)), Typeflag: tar.TypeReg, Uname: "runner", Gname: "runner"}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// regularFiles returns every regular file under dir, keyed by its slash-separated relative
// path, with its mode.
func regularFiles(t *testing.T, dir string) map[string]os.FileMode {
	t.Helper()
	files := map[string]os.FileMode{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = info.Mode()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertFiles(t *testing.T, got map[string]os.FileMode, want ...string) {
	t.Helper()
	for _, rel := range want {
		if _, ok := got[rel]; !ok {
			t.Errorf("%s was not extracted", rel)
		}
	}
	if len(got) != len(want) {
		t.Errorf("extracted %v, want exactly %v", got, want)
	}
}

// TestUploadDownloadRoundTripPlugin publishes a goreleaser dist/ the way
// release-go-plugin.yml does (`gooci upload dist/ <ref>`): one image per archive, under an
// index keyed by platform. Each archive holds LICENSE, README.md and an executable plugin
// at the archive root, as the published plugin artifacts do.
func TestUploadDownloadRoundTripPlugin(t *testing.T) {
	host := newRegistry(t)
	work := t.TempDir()
	t.Chdir(work)
	if err := os.Mkdir("dist", 0o755); err != nil {
		t.Fatal(err)
	}

	archives := []map[string]string{}
	for _, p := range []struct{ goos, goarch, file string }{
		{"linux", "amd64", "plugin-x_Linux_x86_64.tar.gz"},
		{"darwin", "arm64", "plugin-x_Darwin_arm64.tar.gz"},
	} {
		writeTarGz(t, filepath.Join("dist", p.file), []archiveFile{
			{name: "LICENSE", mode: 0o644, body: "license"},
			{name: "README.md", mode: 0o644, body: "readme"},
			{name: "plugin", mode: 0o755, body: "binary-" + p.goos + "-" + p.goarch},
		})
		archives = append(archives, map[string]string{
			"name": p.file, "path": "dist/" + p.file, "goos": p.goos, "goarch": p.goarch, "type": "Archive",
		})
	}
	writeJSON(t, filepath.Join("dist", "artifacts.json"), archives)
	writeJSON(t, filepath.Join("dist", "metadata.json"), map[string]string{"project_name": "plugin-x", "tag": "v0.0.1"})

	ref := host + "/plugin-x:v0.0.1"
	if err := (&uploadRelease{}).run(nil, []string{"dist", ref}); err != nil {
		t.Fatalf("upload: %v", err)
	}

	dest := filepath.Join(work, "out")
	if err := (&downloadRelease{}).run(nil, []string{ref, dest}); err != nil {
		t.Fatalf("download: %v", err)
	}

	files := regularFiles(t, dest)
	assertFiles(t, files, "LICENSE", "README.md", "plugin")
	if mode := files["plugin"]; mode&0o111 == 0 {
		t.Errorf("plugin mode = %v, want it executable", mode)
	}
	// the download picks linux/amd64 from the index by default
	if got, err := os.ReadFile(filepath.Join(dest, "plugin")); err != nil || string(got) != "binary-linux-amd64" {
		t.Errorf("plugin = %q (err=%v), want the linux/amd64 binary", got, err)
	}
}

// TestUploadDownloadRoundTripPolicyBundle publishes an `opa build` bundle the way
// release-policies.yml does (`gooci upload-single bundle.tar.gz <ref>`). The bundle is
// pkg/oci/testdata/opa-bundle.tar.gz, whose entries all start with "/".
func TestUploadDownloadRoundTripPolicyBundle(t *testing.T) {
	bundle, err := filepath.Abs(filepath.Join("..", "pkg", "oci", "testdata", "opa-bundle.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	host := newRegistry(t)
	work := t.TempDir()
	t.Chdir(work)

	ref := host + "/plugin-x-policies:v0.0.1"
	if err := (&uploadSingleArtifact{}).run(nil, []string{bundle, ref}); err != nil {
		t.Fatalf("upload-single: %v", err)
	}

	dest := filepath.Join(work, "out")
	if err := (&downloadRelease{}).run(nil, []string{ref, dest}); err != nil {
		t.Fatalf("download: %v", err)
	}

	assertFiles(t, regularFiles(t, dest), ".manifest", "data.json", "policies/example.rego")
}
