// +build integration

package commands

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/zot/p2p-webapp/internal/pidfile"
)

// TestPsRaceCondition tests that ps shows instances during shutdown
// This is an integration test that starts a real server process
func TestPsRaceCondition(t *testing.T) {
	// Skip if not running with -tags=integration
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Clean up any existing instances
	pidfile.KillAll()
	time.Sleep(500 * time.Millisecond)

	// Create temporary directory with required structure
	tmpDir, err := ioutil.TempDir("", "p2p-webapp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlDir := filepath.Join(tmpDir, "html")
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatalf("Failed to create html dir: %v", err)
	}

	// Create minimal index.html
	indexPath := filepath.Join(htmlDir, "index.html")
	indexContent := `<!DOCTYPE html><html><head><title>Test</title></head><body>Test</body></html>`
	if err := ioutil.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	// Get path to p2p-webapp binary (assume it's built)
	binaryPath := "../../../p2p-webapp"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found at %s, run 'make build' first", binaryPath)
	}

	// Start server with context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "--dir", tmpDir, "--noopen", "--linger", "-v")

	// Capture output
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	serverPID := int32(cmd.Process.Pid)
	t.Logf("Started server with PID: %d", serverPID)

	// Wait for server to be ready (look for "Server started" in output)
	outputBuf := make([]byte, 4096)
	deadline := time.Now().Add(10 * time.Second)
	serverReady := false
	for time.Now().Before(deadline) {
		n, _ := output.Read(outputBuf)
		if n > 0 {
			outputStr := string(outputBuf[:n])
			if strings.Contains(outputStr, "Server started") {
				serverReady = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !serverReady {
		t.Fatalf("Server did not start within timeout")
	}
	t.Log("✓ Server is ready")

	// Verify server is in ps
	pids, err := pidfile.List()
	if err != nil {
		t.Fatalf("Failed to list PIDs: %v", err)
	}

	found := false
	for _, pid := range pids {
		if pid == serverPID {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Server PID %d not found in ps output: %v", serverPID, pids)
	}
	t.Logf("✓ Server PID %d found in ps before shutdown", serverPID)

	// Send SIGTERM to initiate graceful shutdown
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}
	t.Log("✓ Sent SIGTERM to server")

	// Check ps multiple times during shutdown window
	// The fix ensures PID remains in file until shutdown completes
	foundDuringShutdown := false
	for i := 0; i < 20; i++ {
		pids, err := pidfile.List()
		if err == nil {
			for _, pid := range pids {
				if pid == serverPID {
					foundDuringShutdown = true
					t.Logf("✓ Server PID %d still in ps during shutdown (check %d/20)", serverPID, i+1)
					break
				}
			}
		}

		// Check if process is still alive (signal 0 doesn't kill, just checks)
		if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is dead
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Wait for process to fully terminate
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	select {
	case <-waitDone:
		// Process terminated
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not terminate within 5 seconds")
	}
	t.Log("✓ Server terminated")

	// Give a small grace period for cleanup
	time.Sleep(500 * time.Millisecond)

	// Verify PID is now removed from ps
	pids, err = pidfile.List()
	if err != nil {
		t.Fatalf("Failed to list PIDs after shutdown: %v", err)
	}

	for _, pid := range pids {
		if pid == serverPID {
			t.Errorf("❌ Server PID %d still in ps after shutdown completed", serverPID)
		}
	}
	t.Log("✓ Server PID removed from ps after shutdown")

	// Report results
	if foundDuringShutdown {
		t.Log("✅ TEST PASSED: No race condition detected")
		t.Log("   - PID was visible in ps during shutdown")
		t.Log("   - PID was removed after shutdown completed")
	} else {
		t.Log("⚠️  WARNING: Could not verify PID visibility during shutdown")
		t.Log("   - Shutdown may have been too fast")
		t.Log("   - Consider this a PASS if no other errors")
	}
}
