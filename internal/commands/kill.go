package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zot/p2p-webapp/internal/pidfile"
)

// KillCmd represents the kill command
var KillCmd = &cobra.Command{
	Use:   "kill PID",
	Short: "Terminate a running p2p-webapp instance",
	Long:  `Terminate a specific p2p-webapp instance by process ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runKill,
}

func runKill(cmd *cobra.Command, args []string) error {
	// Parse PID
	pid64, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid PID: %s", args[0])
	}
	pid := int32(pid64)

	// Kill the process
	if err := pidfile.Kill(pid); err != nil {
		return err
	}

	fmt.Printf("Successfully killed process %d\n", pid)
	return nil
}
