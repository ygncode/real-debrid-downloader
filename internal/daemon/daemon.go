package daemon

import (
	"os"
	"path/filepath"
	"strconv"
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

// Status returns the current daemon status
func (d *Daemon) Status() (pid int, running bool, err error) {
	pid, running = d.IsRunning()
	return pid, running, nil
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

// filterDaemonArgs removes --daemon and -d flags from args
func filterDaemonArgs(args []string) []string {
	var childArgs []string
	for _, arg := range args {
		if arg != "--daemon" && arg != "-d" {
			childArgs = append(childArgs, arg)
		}
	}
	return childArgs
}
