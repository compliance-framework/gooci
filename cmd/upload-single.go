package cmd

import (
	"errors"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	v2 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path"
	"time"
)

func UploadSingleCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "upload-single [source artifact] [destination registry]",
		Short: "Upload single archive to an OCI registry",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			downloadCmd := &uploadSingleArtifact{}
			err := downloadCmd.run(cmd, args)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return command
}

type uploadSingleArtifact struct {
}

func (d *uploadSingleArtifact) validateArgs(args []string) (*uploadConfig, error) {
	// Validate the first arg is a directory.
	archive := args[0]
	if !path.IsAbs(archive) {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %v", err)
		}
		archive = path.Join(workDir, archive)
	}

	fi, err := os.Stat(archive)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("source archive does not exist: %v", err)
		}

		return nil, fmt.Errorf("failed to stat archive: %v", err)
	}

	if fi.IsDir() {

		return nil, fmt.Errorf("source archive is a directory")
	}

	// Validate the second arg is a valid OCI registry
	repositoryName := args[1]
	tag, err := name.NewTag(repositoryName, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %v", err)
	}

	return &uploadConfig{
		source: archive,
		tag:    tag,
	}, nil
}

func (d *uploadSingleArtifact) run(cmd *cobra.Command, args []string) error {
	config, err := d.validateArgs(args)
	if err != nil {
		return err
	}

	// We'll use the filename as the image title for the moment
	title := path.Base(config.source)

	// We could add more annotations later based on flags or potentially a config file.
	// Right now we push what we know.
	index := mutate.Annotations(empty.Index, map[string]string{
		"org.opencontainers.image.created":  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"org.opencontainers.image.title":    title,
		"org.opencontainers.image.ref.name": config.tag.TagStr(),
		"org.opencontainers.image.version":  config.tag.TagStr(),
	}).(v1.ImageIndex)

	layer, err := tarball.LayerFromFile(config.source)
	if err != nil {
		log.Fatalf("failed to create tarball layer: %v", err)
	}

	// Create an OCI image with the layer
	img, err := mutate.Append(empty.Image, mutate.Addendum{
		MediaType: v2.MediaTypeImageLayerGzip,
		Annotations: map[string]string{
			"org.opencontainers.image.ref.name": title,
		},
		Layer: layer,
	})
	if err != nil {
		return err
	}

	if err := remote.Write(
		config.tag,
		img,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
	); err != nil {
		return err
	}

	index = mutate.AppendManifests(index, mutate.IndexAddendum{
		Add: img,
	})

	err = remote.WriteIndex(config.tag, index, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	return nil
}
