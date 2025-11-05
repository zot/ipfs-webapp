package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/pidfile"
)

var psVerbose bool

// PsCmd represents the ps command
var PsCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running p2p-webapp instances",
	Long:  `List process IDs for all running p2p-webapp instances.`,
	RunE:  runPs,
}

func init() {
	PsCmd.Flags().BoolVarP(&psVerbose, "verbose", "v", false, "Show command line arguments")
}

func runPs(cmd *cobra.Command, args []string) error {
	pids, err := pidfile.List()
	if err != nil {
		return fmt.Errorf("failed to list processes: %w", err)
	}

	if len(pids) == 0 {
		fmt.Println("No running p2p-webapp instances found")
		return nil
	}

	fmt.Printf("Running p2p-webapp instances (%d):\n", len(pids))
	if psVerbose {
		fmt.Println("PID\tCOMMAND")
		for _, pid := range pids {
			_, cmdline, err := pidfile.GetProcessInfo(pid)
			if err != nil {
				fmt.Printf("%d\t<error: %v>\n", pid, err)
			} else if cmdline == "" {
				fmt.Printf("%d\t<no command line available>\n", pid)
			} else {
				fmt.Printf("%d\t%s\n", pid, cmdline)
			}
		}
	} else {
		for _, pid := range pids {
			fmt.Println(pid)
		}
	}

	return nil
}
