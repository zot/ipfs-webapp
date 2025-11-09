package pidfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v3/process"
)

const pidFilePath = "/tmp/.p2p-webapp"

// PIDFile represents the JSON structure of the PID tracking file
type PIDFile struct {
	PIDs []int32 `json:"pids"`
}

var mu sync.Mutex

// withLockedPIDFile handles file opening, locking, reading, verification, and cleanup
func withLockedPIDFile(flags int, fn func(*os.File, []int32) error) error {
	// Ensure directory exists
	dir := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open the file
	file, err := os.OpenFile(pidFilePath, flags, 0644)
	if err != nil {
		return fmt.Errorf("failed to open PID file: %w", err)
	}
	defer file.Close()

	// Lock the file
	if err := lockFile(file); err != nil {
		return err
	}
	defer unlockFile(file)

	// Read current PIDs
	var pids []int32
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() > 0 {
		var pidFile PIDFile
		if err := json.NewDecoder(file).Decode(&pidFile); err == nil {
			pids = pidFile.PIDs
		}
	}

	// Verify PIDs (auto-corrects file if needed)
	validPIDs, err := verifyPIDs(file, pids)
	if err != nil {
		return err
	}

	// Call handler with verified PIDs
	return fn(file, validPIDs)
}

// isIPFSWebappProcess checks if a process is actually an p2p-webapp instance
func isIPFSWebappProcess(pid int32) bool {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return false
	}

	running, err := proc.IsRunning()
	if err != nil || !running {
		return false
	}

	name, err := proc.Name()
	if err != nil {
		return false
	}

	return strings.Contains(name, "p2p-webapp")
}

// writePIDFile writes PIDs to the file
func writePIDFile(file *os.File, pids []int32) error {
	pidFile := PIDFile{PIDs: pids}

	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(&pidFile)
}

// verifyPIDs filters to only valid running processes and rewrites file if changed
func verifyPIDs(file *os.File, pids []int32) ([]int32, error) {
	valid := []int32{}
	for _, pid := range pids {
		if isIPFSWebappProcess(pid) {
			valid = append(valid, pid)
		}
	}

	// Rewrite file if PIDs changed
	if len(valid) != len(pids) {
		if err := writePIDFile(file, valid); err != nil {
			return nil, err
		}
		if _, err := file.Seek(0, 0); err != nil {
			return nil, err
		}
	}

	return valid, nil
}

// Register adds the current process PID to the tracking file
func Register() error {
	mu.Lock()
	defer mu.Unlock()

	currentPID := int32(os.Getpid())

	return withLockedPIDFile(os.O_RDWR|os.O_CREATE, func(file *os.File, pids []int32) error {
		// Add current if not present
		for _, pid := range pids {
			if pid == currentPID {
				return nil
			}
		}

		pids = append(pids, currentPID)
		return writePIDFile(file, pids)
	})
}

// Unregister removes the current process PID from the tracking file
func Unregister() error {
	mu.Lock()
	defer mu.Unlock()

	currentPID := int32(os.Getpid())

	return withLockedPIDFile(os.O_RDWR, func(file *os.File, pids []int32) error {
		filtered := []int32{}
		for _, pid := range pids {
			if pid != currentPID {
				filtered = append(filtered, pid)
			}
		}
		return writePIDFile(file, filtered)
	})
}

// List returns all verified running p2p-webapp PIDs
func List() ([]int32, error) {
	mu.Lock()
	defer mu.Unlock()

	var result []int32
	err := withLockedPIDFile(os.O_RDWR|os.O_CREATE, func(file *os.File, pids []int32) error {
		result = pids
		return nil
	})

	return result, err
}

// Kill terminates a specific PID if it's a valid p2p-webapp process
func Kill(pid int32) error {
	mu.Lock()
	defer mu.Unlock()

	if !isIPFSWebappProcess(pid) {
		return fmt.Errorf("PID %d is not a running p2p-webapp process", pid)
	}

	// Kill the process
	proc, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to get process: %w", err)
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Remove from tracking file (best-effort)
	withLockedPIDFile(os.O_RDWR, func(file *os.File, pids []int32) error {
		filtered := []int32{}
		for _, p := range pids {
			if p != pid {
				filtered = append(filtered, p)
			}
		}
		return writePIDFile(file, filtered)
	})

	return nil
}

// KillAll terminates all tracked p2p-webapp processes
func KillAll() (int, error) {
	mu.Lock()
	defer mu.Unlock()

	var toKill []int32
	err := withLockedPIDFile(os.O_RDWR|os.O_CREATE, func(file *os.File, pids []int32) error {
		toKill = pids
		return writePIDFile(file, []int32{}) // Clear file
	})

	if err != nil {
		return 0, err
	}

	killed := 0
	for _, pid := range toKill {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		if err := proc.Kill(); err == nil {
			killed++
		}
	}

	return killed, nil
}

// GetProcessInfo returns PID and command line for a process
func GetProcessInfo(pid int32) (int32, string, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return 0, "", err
	}

	cmdline, err := proc.Cmdline()
	if err != nil {
		return pid, "", nil
	}

	return pid, cmdline, nil
}
