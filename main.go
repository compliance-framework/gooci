package main

import (
	"github.com/compliance-framework/gooci/cmd"
	"github.com/spf13/cobra"
	"log"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "gooci",
		Short: "gooci handles uploading and downloading of GoReleaser archives to an OCI registry",
	}

	rootCmd.AddCommand(cmd.UploadReleaseCmd())
	rootCmd.AddCommand(cmd.UploadSingleCmd())
	rootCmd.AddCommand(cmd.DownloadReleaseCmd())
	rootCmd.AddCommand(cmd.LoginCmd())
	rootCmd.AddCommand(cmd.LogoutCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
