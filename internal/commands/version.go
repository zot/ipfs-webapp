package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of ipfs-webapp",
	Long:  `Display the current version of ipfs-webapp.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ipfs-webapp version %s\n", Version)
	},
}
