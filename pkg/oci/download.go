package oci

import (
	"archive/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"io"
	"os"
	"path"
	"path/filepath"
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
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
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

func untarToDirectory(destination string, tarReader io.Reader) error {
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
		target := filepath.Join(destination, header.Name)

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
		}
	}
}
