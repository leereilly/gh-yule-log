package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/peterbourgon/ff/v3/ffcli"
)

func main() {
	// Root flags (none currently, but available for future)
	rootFlagSet := flag.NewFlagSet("yule-log", flag.ExitOnError)

	// Run command flags
	runFlagSet := flag.NewFlagSet("yule-log run", flag.ExitOnError)
	runContribs := runFlagSet.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	runGitDir := runFlagSet.String("dir", "", "Git directory for commit ticker (defaults to current dir or YULE_LOG_GIT_DIR)")
	runNoTicker := runFlagSet.Bool("no-ticker", false, "Disable git commit ticker (fire animation only)")

	// Idle command flags
	idleFlagSet := flag.NewFlagSet("yule-log idle", flag.ExitOnError)
	idleTimeout := idleFlagSet.Int("timeout", defaultIdleTimeout, "Idle timeout in seconds before triggering screensaver")
	idleOnce := idleFlagSet.Bool("once", false, "Trigger screensaver immediately and exit")
	idleContribs := idleFlagSet.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	idleNoTicker := idleFlagSet.Bool("no-ticker", false, "Disable git commit ticker")

	// Run subcommand
	runCmd := &ffcli.Command{
		Name:       "run",
		ShortUsage: "yule-log run [flags]",
		ShortHelp:  "Run the screensaver",
		FlagSet:    runFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			return execScreensaver(*runContribs, *runGitDir, *runNoTicker)
		},
	}

	// Idle subcommand
	idleCmd := &ffcli.Command{
		Name:       "idle",
		ShortUsage: "yule-log idle [flags]",
		ShortHelp:  "Run idle watcher daemon",
		FlagSet:    idleFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			return execIdle(*idleTimeout, *idleOnce, *idleContribs, *idleNoTicker)
		},
	}

	// Root command - defaults to running screensaver (backwards compatible)
	rootCmd := &ffcli.Command{
		ShortUsage:  "yule-log [flags] <subcommand>",
		ShortHelp:   "A tmux screensaver with fire animation and git commit ticker",
		LongHelp:    "Controls:\n  Arrow Up/Down   Adjust flame intensity\n  Any other key   Exit screensaver",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{runCmd, idleCmd},
		Exec: func(ctx context.Context, args []string) error {
			// Default behavior: run screensaver (backwards compatible)
			return execScreensaver(false, "", false)
		},
	}

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================================
// Screensaver (run) command
// ============================================================================

func execScreensaver(contribs bool, gitDir string, noTicker bool) error {
	s, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("creating screen: %w", err)
	}
	if err := s.Init(); err != nil {
		return fmt.Errorf("initializing screen: %w", err)
	}
	defer s.Fini()

	s.Clear()
	s.HideCursor()

	width, height := s.Size()
	if width <= 0 || height <= 0 {
		return nil
	}

	size := width * height
	buffer := make([]int, size+width+1)

	var chars []rune
	var styles []tcell.Style

	if contribs {
		// GitHub contribution graph-style glyphs and colors.
		chars = []rune{' ', '⬝', '⬝', '⯀', '⯀', '◼', '◼', '■', '■', '■'}
		styles = []tcell.Style{
			tcell.StyleDefault.Foreground(tcell.ColorBlack),
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(155, 233, 168)),
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(64, 196, 99)),
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(48, 161, 78)),
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(33, 110, 57)),
		}
	} else {
		// Original fire-style glyphs and colors.
		chars = []rune{' ', '.', ':', '^', '*', 'x', 's', 'S', '#', '$'}
		styles = []tcell.Style{
			tcell.StyleDefault.Foreground(tcell.ColorBlack),
			tcell.StyleDefault.Foreground(tcell.ColorMaroon),
			tcell.StyleDefault.Foreground(tcell.ColorRed),
			tcell.StyleDefault.Foreground(tcell.ColorDarkOrange),
			tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true),
		}
	}

	var msgText, metaText string
	var haveTicker bool
	msgRow := height - 2
	metaRow := height - 1
	tickerOffset := 0
	frame := 0
	events := make(chan tcell.Event, 10)

	const (
		maxTickerCommits  = 20
		defaultHeatPower  = 75
		heatSourceDivisor = 6
		minHeat           = 10
		maxHeat           = 85
		minSources        = 1
	)
	heatPower := defaultHeatPower
	heatSources := width / heatSourceDivisor
	if !noTicker {
		msgText, metaText, haveTicker = buildGitTickerText(maxTickerCommits, gitDir)
	}

	go func() {
		for {
			ev := s.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()

	frameDelay := 30 * time.Millisecond

loop:
	for {
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyUp:
					heatPower += 5
					if heatPower > maxHeat {
						heatPower = maxHeat
					}
					heatSources++
					if heatSources > width {
						heatSources = width
					}
				case tcell.KeyDown:
					heatPower -= 5
					if heatPower < minHeat {
						heatPower = minHeat
					}
					if heatSources > minSources {
						heatSources--
					}
				default:
					break loop
				}
			case *tcell.EventResize:
				width, height = s.Size()
				if width <= 0 || height <= 0 {
					break loop
				}
				size = width * height
				buffer = make([]int, size+width+1)
				msgRow = height - 2
				metaRow = height - 1
				heatSources = width / heatSourceDivisor
			}
		default:
		}

		for i := 0; i < heatSources; i++ {
			idx := rand.Intn(width) + width*(height-1)
			if idx >= 0 && idx < len(buffer) {
				buffer[idx] = heatPower
			}
		}

		for i := 0; i < size; i++ {
			b0 := buffer[i]
			b1 := buffer[i+1]
			b2 := buffer[i+width]
			b3 := buffer[i+width+1]
			v := (b0 + b1 + b2 + b3) / 4
			buffer[i] = v
			row := i / width
			col := i % width
			if row >= height || col >= width {
				continue
			}
			if haveTicker && row >= height-2 {
				continue
			}
			var style tcell.Style
			switch {
			case v > 15:
				style = styles[4]
			case v > 9:
				style = styles[3]
			case v > 4:
				style = styles[2]
			default:
				style = styles[1]
			}
			chIdx := v
			if chIdx > 9 {
				chIdx = 9
			}
			if chIdx < 0 {
				chIdx = 0
			}
			s.SetContent(col, row, chars[chIdx], nil, style)
		}

		if haveTicker && height >= 2 && len(msgText) > 0 {
			msgRunes := []rune(msgText)
			metaRunes := []rune(metaText)
			msgLen := len(msgRunes)
			metaLen := len(metaRunes)
			if msgLen > 0 && metaLen > 0 {
				for x := 0; x < width; x++ {
					mi := (tickerOffset + x) % msgLen
					mj := (tickerOffset + x) % metaLen
					mr := msgRunes[mi]
					me := metaRunes[mj]
					s.SetContent(x, msgRow, mr, nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
					s.SetContent(x, metaRow, me, nil, tcell.StyleDefault.Foreground(tcell.ColorWhite))
				}
				if frame%4 == 0 {
					tickerOffset = (tickerOffset + 1) % msgLen
				}
			}
		}

		s.Show()
		time.Sleep(frameDelay)
		frame++
	}

	return nil
}

// ============================================================================
// Idle watcher command
// ============================================================================

const (
	defaultIdleTimeout = 300
	pollInterval       = 5
)

func execIdle(timeout int, once, contribs, noTicker bool) error {
	// Find our own executable path to call "yule-log run"
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}

	if once {
		triggerScreensaver(context.Background(), exePath, contribs, noTicker)
		return nil
	}

	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not running inside tmux")
	}

	// Create context that cancels on SIGINT or SIGTERM for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Yule log idle watcher started (timeout: %ds, poll: %ds)\n", timeout, pollInterval)

	// Use a ticker for controlled polling instead of sleep
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	// Track whether we're waiting for user activity after triggering.
	// This prevents immediate re-triggering when exiting the screensaver,
	// since tmux popup interactions don't update #{client_activity}.
	waitingForActivity := false

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Yule log idle watcher stopped")
			return nil
		case <-ticker.C:
			idleSeconds, err := getClientIdleTime(ctx)
			if err != nil {
				continue
			}

			if waitingForActivity {
				// After triggering, wait for user to become active again
				// before allowing another trigger
				if idleSeconds < timeout {
					waitingForActivity = false
				}
			} else if idleSeconds >= timeout {
				triggerScreensaver(ctx, exePath, contribs, noTicker)
				waitingForActivity = true
			}
		}
	}
}

func getClientIdleTime(ctx context.Context) (int, error) {
	cmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{client_activity}")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("get client activity: %w", err)
	}

	activityStr := strings.TrimSpace(string(out))
	if activityStr == "" {
		return 0, fmt.Errorf("empty activity timestamp")
	}

	activityTime, err := strconv.ParseInt(activityStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse activity timestamp: %w", err)
	}

	now := time.Now().Unix()
	idle := max(int(now-activityTime), 0)
	return idle, nil
}

func triggerScreensaver(ctx context.Context, exePath string, contribs, noTicker bool) {
	// Build command: "yule-log run [flags]"
	args := []string{exePath, "run"}
	if contribs {
		args = append(args, "--contribs")
	}
	if noTicker {
		args = append(args, "--no-ticker")
	}

	// Get the current pane's path for git context
	panePathCmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{pane_current_path}")
	panePathOut, _ := panePathCmd.Output()
	panePath := strings.TrimSpace(string(panePathOut))

	if panePath != "" {
		args = append(args, "--dir", panePath)
	}

	cmdStr := strings.Join(args, " ")

	popupArgs := []string{
		"display-popup",
		"-E",
		"-w", "100%",
		"-h", "100%",
		cmdStr,
	}

	// Note: We don't use CommandContext here because the popup is interactive
	// and should not be killed when the watcher receives a signal - the user
	// should be able to exit it naturally
	cmd := exec.Command("tmux", popupArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Run()
}

// ============================================================================
// Git ticker helpers
// ============================================================================

func parseGitLogToTicker(logOutput string) (string, string, bool) {
	lines := strings.Split(strings.TrimSpace(logOutput), "\n")
	var msgSegs, metaSegs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		_, author, relTime, subject := parts[0], parts[1], parts[2], parts[3]
		message := subject
		meta := "by " + author + " " + relTime
		msgRunes := []rune(message)
		metaRunes := []rune(meta)
		segmentWidth := len(msgRunes)
		if len(metaRunes) > segmentWidth {
			segmentWidth = len(metaRunes)
		}
		segmentWidth += 4
		msgSegs = append(msgSegs, padRight(message, segmentWidth))
		metaSegs = append(metaSegs, padRight(meta, segmentWidth))
	}
	if len(msgSegs) == 0 {
		return "", "", false
	}
	return strings.Join(msgSegs, ""), strings.Join(metaSegs, ""), true
}

func padRight(s string, n int) string {
	rs := []rune(s)
	if len(rs) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(rs))
}

func buildGitTickerText(maxCommits int, gitDir string) (string, string, bool) {
	args := []string{
		"log",
		"-n", strconv.Itoa(maxCommits),
		"--pretty=format:%h%x09%an%x09%ar%x09%s",
	}
	cmd := exec.Command("git", args...)
	if gitDir != "" {
		cmd.Dir = gitDir
	} else if dir := os.Getenv("YULE_LOG_GIT_DIR"); dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", "", false
	}
	return parseGitLogToTicker(string(out))
}
