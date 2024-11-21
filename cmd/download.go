package cmd

import (
	"fmt"
	"github.com/compliance-framework/gooci/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path"
)

func DownloadReleaseCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "download [source oci artifact] [destination]",
		Short: "Download GoReleaser archives to a local directory",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			downloadCmd := &downloadRelease{}
			err := downloadCmd.run(cmd, args)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return command
}

type downloadConfig struct {
	directory string
	source    name.Tag
}

type downloadRelease struct {
}

func (d *downloadRelease) run(cmd *cobra.Command, args []string) error {
	config, err := d.validateArgs(args)
	if err != nil {
		return err
	}

	downloader, err := oci.NewDownloader(config.source, config.directory)
	if err != nil {
		return err
	}

	err = downloader.Download()
	if err != nil {
		return err
	}

	return nil
}

func (d *downloadRelease) validateArgs(args []string) (*downloadConfig, error) {
	// Validate the second arg is a valid OCI registry
	repositoryName := args[0]
	tag, err := name.NewTag(repositoryName, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %v", err)
	}

	// Validate the first arg is a directory.
	destinationDir := args[1]
	if !path.IsAbs(destinationDir) {
		workDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %v", err)
		}
		destinationDir = path.Join(workDir, destinationDir)
	}

	return &downloadConfig{
		directory: destinationDir,
		source:    tag,
	}, nil
}
