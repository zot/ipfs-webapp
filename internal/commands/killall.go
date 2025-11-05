package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/pidfile"
)

// KillAllCmd represents the killall command
var KillAllCmd = &cobra.Command{
	Use:   "killall",
	Short: "Terminate all running p2p-webapp instances",
	Long:  `Terminate all running p2p-webapp instances.`,
	RunE:  runKillAll,
}

func runKillAll(cmd *cobra.Command, args []string) error {
	killed, err := pidfile.KillAll()
	if err != nil {
		return fmt.Errorf("failed to kill processes: %w", err)
	}

	if killed == 0 {
		fmt.Println("No running p2p-webapp instances found")
	} else {
		fmt.Printf("Successfully killed %d process(es)\n", killed)
	}

	return nil
}
