package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Daemon manages the daemon lifecycle (PID file, logs, process control)
type Daemon struct {
	PIDFile string
	LogFile string
}

// New creates a new Daemon instance with default paths in ~/.rd-downloader/
func New() *Daemon {
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".rd-downloader")
	os.MkdirAll(baseDir, 0755)

	return &Daemon{
		PIDFile: filepath.Join(baseDir, "rd-downloader.pid"),
		LogFile: filepath.Join(baseDir, "rd-downloader.log"),
	}
}

// Start forks the process into the background
// Returns true if this is the parent process (should exit)
// Returns false if this is the child process (should continue)
func (d *Daemon) Start(args []string) error {
	// Check if already running
	if pid, running := d.IsRunning(); running {
		return fmt.Errorf("rd-downloader is already running (PID: %d)", pid)
	}

	// Clean up stale PID file if exists
	d.cleanStalePID()

	// Filter out --daemon flag from args for the child process
	var childArgs []string
	for _, arg := range args {
		if arg != "--daemon" && arg != "-d" {
			childArgs = append(childArgs, arg)
		}
	}

	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Open log file for output
	logFile, err := os.OpenFile(d.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create the child process
	cmd := exec.Command(executable, childArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Start the child process
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file
	if err := d.writePID(cmd.Process.Pid); err != nil {
		// Try to kill the child if we can't write PID
		cmd.Process.Kill()
		logFile.Close()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	logFile.Close()
	return nil
}

// Stop stops the running daemon
func (d *Daemon) Stop() error {
	pid, running := d.IsRunning()
	if !running {
		// Clean up stale PID file if exists
		d.cleanStalePID()
		return fmt.Errorf("rd-downloader is not running")
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		d.removePID()
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		d.removePID()
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Remove PID file
	d.removePID()

	return nil
}

// Status returns the current daemon status
func (d *Daemon) Status() (pid int, running bool, err error) {
	pid, running = d.IsRunning()
	return pid, running, nil
}

// IsRunning checks if the daemon is currently running
func (d *Daemon) IsRunning() (pid int, running bool) {
	pid, err := d.readPID()
	if err != nil {
		return 0, false
	}

	// Check if process is actually running
	process, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return pid, false
	}

	return pid, true
}

// GetLogFile returns the path to the log file
func (d *Daemon) GetLogFile() string {
	return d.LogFile
}

// writePID writes the process ID to the PID file
func (d *Daemon) writePID(pid int) error {
	return os.WriteFile(d.PIDFile, []byte(strconv.Itoa(pid)), 0644)
}

// readPID reads the process ID from the PID file
func (d *Daemon) readPID() (int, error) {
	data, err := os.ReadFile(d.PIDFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// removePID removes the PID file
func (d *Daemon) removePID() {
	os.Remove(d.PIDFile)
}

// cleanStalePID removes the PID file if the process is not running
func (d *Daemon) cleanStalePID() {
	if _, err := os.Stat(d.PIDFile); err == nil {
		if _, running := d.IsRunning(); !running {
			d.removePID()
		}
	}
}
