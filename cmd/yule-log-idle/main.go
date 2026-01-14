package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultIdleTimeout = 300 // 5 minutes in seconds
	pollInterval       = 5   // seconds between activity checks
)

func main() {
	timeout := flag.Int("timeout", defaultIdleTimeout, "Idle timeout in seconds before triggering screensaver")
	once := flag.Bool("once", false, "Trigger screensaver immediately and exit (for manual trigger)")
	contribs := flag.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	noTicker := flag.Bool("no-ticker", false, "Disable git commit ticker")
	flag.Parse()

	// Find the yule-log binary relative to this executable
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "finding executable path: %v\n", err)
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)
	yuleLogBin := filepath.Join(exeDir, "yule-log")

	// Fallback: try to find yule-log in same directory or PATH
	if _, err := os.Stat(yuleLogBin); os.IsNotExist(err) {
		// Try relative to working directory
		if wd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(wd, "bin", "yule-log")
			if _, err := os.Stat(candidate); err == nil {
				yuleLogBin = candidate
			}
		}
	}

	// If --once flag, trigger immediately
	if *once {
		triggerScreensaver(yuleLogBin, *contribs, *noTicker)
		return
	}

	// Check if we're inside tmux
	if os.Getenv("TMUX") == "" {
		fmt.Fprintf(os.Stderr, "not running inside tmux\n")
		os.Exit(1)
	}

	// Main idle monitoring loop
	fmt.Printf("Yule log idle watcher started (timeout: %ds)\n", *timeout)
	for {
		idleSeconds, err := getClientIdleTime()
		if err != nil {
			// Not fatal, might be a transient error
			time.Sleep(time.Duration(pollInterval) * time.Second)
			continue
		}

		if idleSeconds >= *timeout {
			triggerScreensaver(yuleLogBin, *contribs, *noTicker)
			// After screensaver exits (user pressed a key), reset the loop
			// The activity timestamp will be fresh
		}

		time.Sleep(time.Duration(pollInterval) * time.Second)
	}
}

// getClientIdleTime returns how many seconds since last client activity
func getClientIdleTime() (int, error) {
	// Get current timestamp and client activity timestamp from tmux
	cmd := exec.Command("tmux", "display-message", "-p", "#{client_activity}")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get client activity: %w", err)
	}

	activityStr := strings.TrimSpace(string(out))
	if activityStr == "" {
		return 0, fmt.Errorf("empty activity timestamp")
	}

	activityTime, err := strconv.ParseInt(activityStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse activity timestamp: %w", err)
	}

	now := time.Now().Unix()
	idle := int(now - activityTime)
	if idle < 0 {
		idle = 0
	}

	return idle, nil
}

// triggerScreensaver launches the yule-log screensaver in a tmux popup
func triggerScreensaver(yuleLogBin string, contribs, noTicker bool) {
	// Build the command to run in the popup
	args := []string{yuleLogBin}
	if contribs {
		args = append(args, "--contribs")
	}
	if noTicker {
		args = append(args, "--no-ticker")
	}
	cmdStr := strings.Join(args, " ")

	// Get the current pane's path for git context
	panePathCmd := exec.Command("tmux", "display-message", "-p", "#{pane_current_path}")
	panePathOut, err := panePathCmd.Output()
	if err != nil {
		// Non-fatal: continue without git context if we can't get pane path
		panePathOut = nil
	}
	panePath := strings.TrimSpace(string(panePathOut))

	// Add --dir flag if we have a pane path
	if panePath != "" {
		cmdStr = fmt.Sprintf("%s --dir %q", cmdStr, panePath)
	}

	// Launch the screensaver in a full-screen tmux popup
	// -E: Close popup when command exits
	// -w 100% -h 100%: Full screen
	popupArgs := []string{
		"display-popup",
		"-E",         // Exit when command exits
		"-w", "100%", // Full width
		"-h", "100%", // Full height
		cmdStr,
	}

	cmd := exec.Command("tmux", popupArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Run and wait for completion (user presses a key to exit).
	// Screensaver exit errors are non-critical; user exits with any key.
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "screensaver exited with error: %v\n", err)
	}
}
