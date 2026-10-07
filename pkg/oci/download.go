package oci

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Downloader struct {
	destination string

	// Reference is the processed OCI Name of the Source
	reference name.Tag
}

func NewDownloader(source name.Tag, destination string) (Downloader, error) {
	return Downloader{
		destination: destination,
		reference:   source,
	}, nil
}

// Download executes the download of the OCI artifact into memory, untars it and write it to a directory.
// This will need to be updated at some point when we are working with OCI artifacts rather than images,
// to take slightly different actions based on the artifact type we receive from the registry (image / binary / fs)
func (dl *Downloader) Download(option ...remote.Option) error {
	opts := []remote.Option{
		remote.WithAuthFromKeychain(ECRKeychain()),
	}
	opts = append(opts, option...)
	img, err := remote.Image(dl.reference, opts...)
	if err != nil {
		return err
	}

	outputDir := dl.destination
	if !path.IsAbs(outputDir) {
		workdDir, err := os.Getwd()
		if err != nil {
			return err
		}
		outputDir = path.Join(workdDir, outputDir)
	}

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		layerReader, err := layer.Uncompressed()
		if err != nil {
			return err
		}
		err = untarToDirectory(outputDir, layerReader)
		if err != nil {
			return err
		}
	}

	return nil
}

// resolveInDestination joins name onto destination and returns the result, or an error when
// name is absolute or the joined path falls outside destination.
func resolveInDestination(destination, name string) (string, error) {
	if path.IsAbs(name) || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("refusing to extract %q: absolute paths are not allowed", name)
	}

	target := filepath.Join(destination, name)
	if !isInDestination(destination, target) {
		return "", fmt.Errorf("refusing to extract %q: path is outside the destination directory %q", name, destination)
	}

	return target, nil
}

// isInDestination reports whether the cleaned target is destination itself or inside it.
func isInDestination(destination, target string) bool {
	destination = filepath.Clean(destination)
	target = filepath.Clean(target)
	if target == destination {
		return true
	}

	prefix := destination
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	return strings.HasPrefix(target, prefix)
}

// untarToDirectory extracts directories and regular files from tarReader into destination.
// Every entry must stay inside destination: absolute names, names that escape it, and
// symlinks or hardlinks whose targets escape it are rejected with an error. Links that stay
// inside destination, device files and FIFOs are not created.
func untarToDirectory(destination string, tarReader io.Reader) error {
	destination = filepath.Clean(destination)
	tr := tar.NewReader(tarReader)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target, err := resolveInDestination(destination, header.Name)
		if err != nil {
			return err
		}

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			targetDir := filepath.Dir(target)
			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				if err := os.MkdirAll(targetDir, 0755); err != nil {
					return err
				}
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			err = f.Close()
			if err != nil {
				return err
			}

		// links are not created, but one whose target escapes the destination marks the
		// archive as unsafe, so it is rejected rather than silently skipped.
		case tar.TypeSymlink:
			// a symlink target is relative to the directory holding the link
			if path.IsAbs(header.Linkname) || filepath.IsAbs(header.Linkname) || filepath.VolumeName(header.Linkname) != "" {
				return fmt.Errorf("refusing to extract symlink %q: absolute target %q is not allowed", header.Name, header.Linkname)
			}
			if !isInDestination(destination, filepath.Join(filepath.Dir(target), header.Linkname)) {
				return fmt.Errorf("refusing to extract symlink %q: target %q is outside the destination directory %q", header.Name, header.Linkname, destination)
			}

		case tar.TypeLink:
			// a hardlink target is relative to the root of the archive
			if _, err := resolveInDestination(destination, header.Linkname); err != nil {
				return fmt.Errorf("refusing to extract hardlink %q: %w", header.Name, err)
			}

		// device files, FIFOs and any other entry types are skipped
		default:
		}
	}
}
