package oci

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	body     string
}

func buildTar(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Linkname: e.linkname,
			Mode:     0o644,
		}
		if e.typeflag == tar.TypeDir {
			hdr.Mode = 0o755
		}
		if e.typeflag == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if e.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("write body %q: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return &buf
}

// newDestination returns a destination nested two levels inside a temporary root, so an
// entry that escapes the destination still lands inside the test's temporary directory.
func newDestination(t *testing.T) (root, dest string) {
	t.Helper()
	root = t.TempDir()
	dest = filepath.Join(root, "cache", "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, dest
}

// filesOutside lists every path under root that is not dest or inside it.
func filesOutside(t *testing.T, root, dest string) []string {
	t.Helper()
	var outside []string
	err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == root || p == filepath.Dir(dest) || p == dest {
			return nil
		}
		if !strings.HasPrefix(p, dest+string(filepath.Separator)) {
			outside = append(outside, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return outside
}

func TestUntarToDirectoryRejectsEntriesOutsideDestination(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name:    "parent directory file",
			entries: []tarEntry{{name: "../../escaped", typeflag: tar.TypeReg, body: "x"}},
		},
		{
			name:    "nested parent directory file",
			entries: []tarEntry{{name: "a/../../../escaped", typeflag: tar.TypeReg, body: "x"}},
		},
		{
			name:    "sibling sharing the destination prefix",
			entries: []tarEntry{{name: "../dest-evil/escaped", typeflag: tar.TypeReg, body: "x"}},
		},
		{
			name:    "parent directory dir",
			entries: []tarEntry{{name: "../../escaped-dir/", typeflag: tar.TypeDir}},
		},
		{
			name:    "absolute path",
			entries: []tarEntry{{name: "/escaped", typeflag: tar.TypeReg, body: "x"}},
		},
		{
			name:    "symlink to a parent directory",
			entries: []tarEntry{{name: "link", typeflag: tar.TypeSymlink, linkname: "../../outside"}},
		},
		{
			name:    "symlink to an absolute path",
			entries: []tarEntry{{name: "link", typeflag: tar.TypeSymlink, linkname: "/etc"}},
		},
		{
			name:    "hardlink to a parent directory",
			entries: []tarEntry{{name: "link", typeflag: tar.TypeLink, linkname: "../../outside"}},
		},
		{
			name:    "hardlink to an absolute path",
			entries: []tarEntry{{name: "link", typeflag: tar.TypeLink, linkname: "/etc/passwd"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dest := newDestination(t)

			err := untarToDirectory(dest, buildTar(t, tt.entries))
			if err == nil {
				t.Errorf("expected an error for an entry outside the destination, got nil")
			}
			if outside := filesOutside(t, root, dest); len(outside) > 0 {
				t.Errorf("files were written outside the destination: %v", outside)
			}
		})
	}
}

func TestUntarToDirectoryExtractsEntriesInsideDestination(t *testing.T) {
	root, dest := newDestination(t)

	entries := []tarEntry{
		{name: "./", typeflag: tar.TypeDir},
		{name: "policies/", typeflag: tar.TypeDir},
		{name: "policies/main.rego", typeflag: tar.TypeReg, body: "package main"},
		{name: "./plugin", typeflag: tar.TypeReg, body: "binary"},
		{name: "nested/dir/file", typeflag: tar.TypeReg, body: "nested"},
		{name: "nested/../flattened", typeflag: tar.TypeReg, body: "flat"},
		// Links and special files inside the destination are skipped, as before.
		{name: "policies/link", typeflag: tar.TypeSymlink, linkname: "main.rego"},
		{name: "hardlink", typeflag: tar.TypeLink, linkname: "policies/main.rego"},
		{name: "fifo", typeflag: tar.TypeFifo},
		{name: "chardev", typeflag: tar.TypeChar},
		{name: "blockdev", typeflag: tar.TypeBlock},
	}

	if err := untarToDirectory(dest, buildTar(t, entries)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"policies/main.rego": "package main",
		"plugin":             "binary",
		"nested/dir/file":    "nested",
		"flattened":          "flat",
	}
	for rel, body := range want {
		got, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			t.Errorf("read %s: %v", rel, err)
			continue
		}
		if string(got) != body {
			t.Errorf("%s = %q, want %q", rel, got, body)
		}
	}

	for _, rel := range []string{"policies/link", "hardlink", "fifo", "chardev", "blockdev"} {
		if _, err := os.Lstat(filepath.Join(dest, rel)); !os.IsNotExist(err) {
			t.Errorf("%s should not have been created (err=%v)", rel, err)
		}
	}

	if outside := filesOutside(t, root, dest); len(outside) > 0 {
		t.Errorf("files were written outside the destination: %v", outside)
	}
}

func TestUntarToDirectoryRelativeAndAbsoluteDestinations(t *testing.T) {
	tests := []struct {
		name        string
		destination func(cwd string) string
		// dir is where the destination lives, relative to cwd
		dir string
	}{
		{name: "dot", destination: func(string) string { return "." }, dir: "."},
		{name: "dot slash", destination: func(string) string { return "./" }, dir: "."},
		{name: "relative", destination: func(string) string { return "out" }, dir: "out"},
		{name: "dot slash relative", destination: func(string) string { return "./out" }, dir: "out"},
		{name: "absolute", destination: func(cwd string) string { return filepath.Join(cwd, "abs") }, dir: "abs"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/normal entry", func(t *testing.T) {
			cwd := filepath.Join(t.TempDir(), "work")
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)

			entries := []tarEntry{{name: "plugin", typeflag: tar.TypeReg, body: "binary"}}
			if err := untarToDirectory(tt.destination(cwd), buildTar(t, entries)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(cwd, tt.dir, "plugin"))
			if err != nil {
				t.Fatalf("read plugin: %v", err)
			}
			if string(got) != "binary" {
				t.Errorf("plugin = %q, want %q", got, "binary")
			}
		})

		t.Run(tt.name+"/parent entry", func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "work")
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)

			dest := filepath.Join(cwd, tt.dir)
			// where "../x" would land if it escaped the destination
			escaped := filepath.Join(dest, "..", "x")

			entries := []tarEntry{{name: "../x", typeflag: tar.TypeReg, body: "x"}}
			if err := untarToDirectory(tt.destination(cwd), buildTar(t, entries)); err == nil {
				t.Errorf("expected an error for %q, got nil", "../x")
			}
			if _, err := os.Lstat(escaped); !os.IsNotExist(err) {
				t.Errorf("%s should not have been created (err=%v)", escaped, err)
			}
		})
	}
}
