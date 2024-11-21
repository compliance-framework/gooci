package cmd

import (
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/spf13/cobra"
)

func LogoutCmd() *cobra.Command {

	// For simplicity, and because we use the library quite a bit, we'll just replicate the crane logout CMD
	// and update the usage instructions to match the gooci style

	command := craneCmd.NewCmdAuthLogout()
	command.Example = ""

	return command
}
