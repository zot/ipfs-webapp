package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

// VersionCmd represents the version command
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of p2p-webapp",
	Long:  `Display the current version of p2p-webapp.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("p2p-webapp version %s\n", Version)
	},
}
