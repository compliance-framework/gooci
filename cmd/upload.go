package cmd

import (
	"errors"
	"fmt"
	"github.com/compliance-framework/gooci/pkg/metadata"
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

func UploadReleaseCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "upload [source dir] [destination registry]",
		Short: "Upload GoReleaser archives to an OCI registry",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			downloadCmd := &uploadRelease{}
			err := downloadCmd.run(cmd, args)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return command
}

type uploadConfig struct {
	source string
	tag    name.Tag
}

type uploadRelease struct {
}

func (d *uploadRelease) validateArgs(args []string) (*uploadConfig, error) {
	// Validate the first arg is a directory.
	archiveDir := args[0]
	if !path.IsAbs(archiveDir) {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %v", err)
		}
		archiveDir = path.Join(workDir, archiveDir)
	}

	fi, err := os.Stat(archiveDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("source directory does not exist: %v", err)
		}

		return nil, fmt.Errorf("failed to stat archive directory: %v", err)
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("source directory is not a directory")
	}

	// Validate the second arg is a valid OCI registry
	repositoryName := args[1]
	tag, err := name.NewTag(repositoryName, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %v", err)
	}

	return &uploadConfig{
		source: archiveDir,
		tag:    tag,
	}, nil
}

func (d *uploadRelease) run(cmd *cobra.Command, args []string) error {
	config, err := d.validateArgs(args)
	if err != nil {
		return err
	}

	data, err := metadata.New(config.source)
	if err != nil {
		return err
	}

	wordir, err := os.Getwd()
	if err != nil {
		return err
	}

	// We could add more annotations later based on flags or potentially a config file.
	// Right now we push what we know.
	index := mutate.Annotations(empty.Index, map[string]string{
		"org.opencontainers.image.created":     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"org.opencontainers.image.title":       data.Metadata.ProjectName,
		"org.opencontainers.image.description": data.Metadata.Description,
		"org.opencontainers.image.ref.name":    config.tag.TagStr(),
		"org.opencontainers.image.version":     config.tag.TagStr(),
	}).(v1.ImageIndex)

	for _, archive := range *data.GetArchives() {

		layer, err := tarball.LayerFromFile(path.Join(wordir, archive.Path))
		if err != nil {
			log.Fatalf("failed to create tarball layer: %v", err)
		}

		// Create an OCI image with the layer
		img, err := mutate.Append(empty.Image, mutate.Addendum{
			MediaType: v2.MediaTypeImageLayerGzip,
			Annotations: map[string]string{
				"org.opencontainers.image.ref.name": archive.Name,
			},
			Layer: layer,
		})
		if err != nil {
			return err
		}

		platformDesc := v1.Descriptor{
			Platform: &v1.Platform{
				OS:           archive.Goos,
				Architecture: archive.Goarch,
			},
		}

		if err := remote.Write(
			config.tag,
			img,
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
			remote.WithPlatform(*platformDesc.Platform),
		); err != nil {
			return err
		}

		index = mutate.AppendManifests(index, mutate.IndexAddendum{
			Add:        img,
			Descriptor: platformDesc,
		})
	}

	err = remote.WriteIndex(config.tag, index, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	return nil
}
