package cmd

import (
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/spf13/cobra"
)

func LoginCmd() *cobra.Command {

	// For simplicity, and because we use the library quite a bit, we'll just replicate the crane login CMD
	// and update the usage instructions to match the gooci style

	command := craneCmd.NewCmdAuthLogin()
	command.Short = "Login to a remote registry"
	command.Example = ""

	return command
}
