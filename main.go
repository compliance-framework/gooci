package main

import (
	"github.com/compliance-framework/goreleaser-oci/cmd"
	"github.com/spf13/cobra"
	"log"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "gooci",
		Short: "gooci handles uploading and downloading of GoReleaser archives to an OCI registry",
	}

	rootCmd.AddCommand(cmd.UploadReleaseCmd())
	rootCmd.AddCommand(cmd.DownloadReleaseCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
