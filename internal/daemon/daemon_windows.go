//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Start forks the process into the background
func (d *Daemon) Start(args []string) error {
	// Check if already running
	if pid, running := d.IsRunning(); running {
		return fmt.Errorf("rd-downloader is already running (PID: %d)", pid)
	}

	// Clean up stale PID file if exists
	d.cleanStalePID()

	// Filter out --daemon flag from args for the child process
	childArgs := filterDaemonArgs(args)

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

	// Detach from parent process (Windows-specific)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
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

	// On Windows, use Kill() as there's no SIGTERM equivalent
	if err := process.Kill(); err != nil {
		d.removePID()
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Remove PID file
	d.removePID()

	return nil
}

// IsRunning checks if the daemon is currently running
func (d *Daemon) IsRunning() (pid int, running bool) {
	pid, err := d.readPID()
	if err != nil {
		return 0, false
	}

	// On Windows, we check if the process exists by trying to open it
	process, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}

	// On Windows, FindProcess always succeeds, so we try to signal it
	// This will fail if the process doesn't exist
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return pid, false
	}

	return pid, true
}
