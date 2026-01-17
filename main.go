package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"io"
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
	"golang.org/x/term"

	"yule-log/internal/fire"
	"yule-log/internal/lock"
)

// ---- Constants

const (
	// Timing
	frameDelay         = 30 * time.Millisecond
	defaultIdleTimeout = 300
	pollInterval       = 5

	// Fire simulation
	maxTickerCommits  = 20
	defaultHeatPower  = 75
	heatSourceDivisor = 6
	minHeat           = 10
	maxHeat           = 85
	minSources        = 1

	// Heat value thresholds for color selection
	heatThresholdHigh   = 15
	heatThresholdMedium = 9
	heatThresholdLow    = 4
	heatThresholdMin    = 1

	// Color shift thresholds
	colorShiftBaseHeat = 18
	colorShiftMaxHeat  = 38

	// Terminal input byte values
	byteEscape         = 0x1b
	byteCtrlC          = 0x03
	byteBackspace      = 0x7f
	byteDelete         = 0x08
	bytePrintableStart = 0x20
	bytePrintableEnd   = 0x7f
)

// Mode represents the screensaver operating mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModePlayground
	ModeLock
)

// ---- Visual Themes

type theme struct {
	chars []rune
}

var (
	fireTheme = theme{
		chars: []rune{' ', '.', ':', '^', '*', 'x', 's', 'S', '#', '$'},
	}

	contribTheme = theme{
		chars: []rune{' ', '⬝', '⬝', '⯀', '⯀', '◼', '◼', '■', '■', '■'},
	}
)

// ---- Screensaver Configuration & State

type screensaverConfig struct {
	mode     Mode
	contribs bool
	gitDir   string
	noTicker bool
	cooldown fire.CooldownSpeed
}

func (c screensaverConfig) theme() theme {
	if c.contribs {
		return contribTheme
	}
	return fireTheme
}

type screensaver struct {
	cfg    screensaverConfig
	screen tcell.Screen
	theme  theme

	// Dimensions
	width, height int

	// Fire state
	buffer      []int
	heatPower   int
	heatSources int

	// Ticker state
	msgText, metaText string
	haveTicker        bool
	tickerOffset      int
	frame             int

	// Interactive state (nil in normal mode)
	visualState *fire.VisualState
	inputBuffer *lock.SecureBuffer

	// Input timeout (frames since last input, for clearing password)
	framesSinceInput int

	// Wrong password animation (frames remaining, fades from 1.0 to 0.0)
	wrongPasswordFrames int

	// Event channel
	events   chan tcell.Event
	pollDone chan struct{}
}

func newScreensaver(cfg screensaverConfig) (*screensaver, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("creating screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return nil, fmt.Errorf("initializing screen: %w", err)
	}

	s := &screensaver{
		cfg:       cfg,
		screen:    screen,
		theme:     cfg.theme(),
		heatPower: defaultHeatPower,
		events:    make(chan tcell.Event, 10),
		pollDone:  make(chan struct{}),
	}

	s.visualState = fire.NewVisualStateWithPreset(cfg.cooldown)
	s.heatPower = s.visualState.EffectiveHeatPower()

	if cfg.mode == ModeLock {
		s.inputBuffer = lock.NewSecureBuffer()
	}

	s.resize()
	s.loadTicker()

	return s, nil
}

func (s *screensaver) close() {
	if s.inputBuffer != nil {
		s.inputBuffer.Destroy()
	}
	s.screen.Fini()

	// Wait for pollEvents goroutine to finish
	select {
	case <-s.pollDone:
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *screensaver) resize() {
	s.width, s.height = s.screen.Size()
	if s.width <= 0 || s.height <= 0 {
		return
	}
	size := s.width * s.height
	// Extra space (width+1) for fire propagation lookups: i+1, i+width, i+width+1
	s.buffer = make([]int, size+s.width+1)
	s.heatSources = s.width / heatSourceDivisor
}

func (s *screensaver) loadTicker() {
	if s.cfg.noTicker {
		return
	}
	s.msgText, s.metaText, s.haveTicker = buildGitTickerText(maxTickerCommits, s.cfg.gitDir)
}

// ---- Event Handling

type action int

const (
	actionNone action = iota
	actionExit
	actionResize
)

func (s *screensaver) handleEvent(ev tcell.Event) action {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		s.resize()
		if s.width <= 0 || s.height <= 0 {
			return actionExit
		}
		return actionResize

	case *tcell.EventKey:
		return s.handleKey(ev)
	}
	return actionNone
}

func (s *screensaver) handleKey(ev *tcell.EventKey) action {
	// Feed fire in interactive modes
	if s.visualState != nil {
		s.visualState.OnKeyPress()
		s.heatPower = s.visualState.EffectiveHeatPower()
	}

	switch s.cfg.mode {
	case ModeLock:
		return s.handleKeyLock(ev)
	case ModePlayground:
		return s.handleKeyPlayground(ev)
	default:
		return s.handleKeyNormal(ev)
	}
}

func (s *screensaver) handleKeyNormal(ev *tcell.EventKey) action {
	switch ev.Key() {
	case tcell.KeyEscape:
		return actionExit
	case tcell.KeyUp, tcell.KeyDown:
		return actionNone // Fire burst handled by visualState.OnKeyPress()
	default:
		return actionExit
	}
}

func (s *screensaver) handleKeyPlayground(ev *tcell.EventKey) action {
	if ev.Key() == tcell.KeyEscape {
		return actionExit
	}
	return actionNone
}

// wrongPasswordDuration is frames for wrong password red animation (~2 sec).
const wrongPasswordDuration = 67 // ~2 sec at 30ms/frame

// inputTimeoutFrames is how long to wait before clearing password input.
const inputTimeoutFrames = 200 // ~6 seconds at 30ms/frame

func (s *screensaver) handleKeyLock(ev *tcell.EventKey) action {
	// Reset input timeout on any keypress
	s.framesSinceInput = 0
	switch ev.Key() {
	case tcell.KeyEnter:
		if s.tryUnlock() {
			return actionExit // Just exit, no flash
		}
		// Wrong password - red spike animation
		s.wrongPasswordFrames = wrongPasswordDuration
		s.inputBuffer.Clear()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.inputBuffer.Backspace()
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight:
		s.inputBuffer.AppendString(lock.ArrowKeyMarker(ev.Key()))
	case tcell.KeyRune:
		s.inputBuffer.AppendRune(ev.Rune())
	}
	return actionNone
}

func (s *screensaver) tryUnlock() bool {
	password := s.inputBuffer.Bytes()
	defer lock.ClearBytes(password)

	valid, err := lock.CheckPassword(password)
	if err != nil || !valid {
		s.inputBuffer.Clear()
		return false
	}
	return true
}

// ---- Rendering

func (s *screensaver) run() error {
	s.screen.Clear()
	s.screen.HideCursor()

	if s.width <= 0 || s.height <= 0 {
		return nil
	}

	go s.pollEvents()

	for {
		if done := s.processEvents(); done {
			return nil
		}
		s.updateVisualState()
		s.renderFrame()
		time.Sleep(frameDelay)
		s.frame++
	}
}

// pollEvents reads events until the screen is finalized.
// When screen.Fini() is called (in close()), PollEvent returns nil, ending this goroutine.
func (s *screensaver) pollEvents() {
	defer close(s.pollDone)
	for {
		ev := s.screen.PollEvent()
		if ev == nil {
			return
		}
		s.events <- ev
	}
}

func (s *screensaver) processEvents() bool {
	for {
		select {
		case ev := <-s.events:
			if s.handleEvent(ev) == actionExit {
				return true
			}
		default:
			return false
		}
	}
}

func (s *screensaver) updateVisualState() {
	if s.visualState == nil {
		return
	}

	s.visualState.OnFrame()
	s.heatPower = s.visualState.EffectiveHeatPower()

	// Decrement wrong password animation
	if s.wrongPasswordFrames > 0 {
		s.wrongPasswordFrames--
	}

	// Track input timeout and clear password after timeout
	if s.cfg.mode == ModeLock && s.inputBuffer != nil && s.inputBuffer.Len() > 0 {
		s.framesSinceInput++
		if s.framesSinceInput >= inputTimeoutFrames {
			s.inputBuffer.Clear()
			s.framesSinceInput = 0
		}
	}
}

func (s *screensaver) renderFrame() {
	s.generateHeat()
	s.renderFire()
	s.renderPasswordIndicator()
	s.renderTicker()
	s.screen.Show()
}

// renderPasswordIndicator displays asterisks for password input in lock mode.
func (s *screensaver) renderPasswordIndicator() {
	if s.cfg.mode != ModeLock || s.inputBuffer == nil {
		return
	}

	count := s.inputBuffer.VisualLen()
	if count == 0 {
		return
	}

	// Render at top-left: "> ****"
	dimStyle := tcell.StyleDefault.Dim(true)
	col := 0
	s.screen.SetContent(col, 0, '>', nil, dimStyle)
	col++
	s.screen.SetContent(col, 0, ' ', nil, tcell.StyleDefault)
	col++

	for i := 0; i < count && col < s.width; i++ {
		s.screen.SetContent(col, 0, '*', nil, dimStyle)
		col++
	}
}

func (s *screensaver) generateHeat() {
	bottomRow := s.width * (s.height - 1)
	for i := 0; i < s.heatSources; i++ {
		idx := rand.Intn(s.width) + bottomRow
		if idx >= 0 && idx < len(s.buffer) {
			s.buffer[idx] = s.heatPower
		}
	}
}

func (s *screensaver) renderFire() {
	size := s.width * s.height
	tickerRows := 0
	if s.haveTicker {
		tickerRows = 2
	}

	for i := 0; i < size; i++ {
		s.buffer[i] = (s.buffer[i] + s.buffer[i+1] + s.buffer[i+s.width] + s.buffer[i+s.width+1]) / 4

		row, col := i/s.width, i%s.width
		if row >= s.height || col >= s.width || row >= s.height-tickerRows {
			continue
		}

		v := s.buffer[i]
		style := s.styleForValue(v)
		char := s.theme.chars[clamp(v, 0, 9)]
		s.screen.SetContent(col, row, char, nil, style)
	}
}

// Base RGB colors for fire (matching the theme visually).
// Using consistent RGB values ensures smooth transitions.
var fireBaseColors = []struct{ r, g, b uint8 }{
	{128, 0, 0},    // Maroon (dark, low heat)
	{200, 50, 0},   // Dark red-orange
	{255, 100, 0},  // Orange
	{255, 160, 0},  // Bright orange
	{255, 200, 50}, // Yellow-orange (high heat)
}

func (s *screensaver) styleForValue(v int) tcell.Style {
	// Use RGB-based colors for smooth transitions in all modes
	return s.rgbStyle(v)
}

// rgbStyle returns RGB-based style with color derived from cell heat.
// Both height and color use the same source (cell heat v) so they correlate.
func (s *screensaver) rgbStyle(v int) tcell.Style {
	// Select base color from heat value
	var r, g, b uint8
	switch {
	case v > heatThresholdHigh:
		r, g, b = fireBaseColors[4].r, fireBaseColors[4].g, fireBaseColors[4].b
	case v > heatThresholdMedium:
		r, g, b = fireBaseColors[3].r, fireBaseColors[3].g, fireBaseColors[3].b
	case v > heatThresholdLow:
		r, g, b = fireBaseColors[2].r, fireBaseColors[2].g, fireBaseColors[2].b
	case v > heatThresholdMin:
		r, g, b = fireBaseColors[1].r, fireBaseColors[1].g, fireBaseColors[1].b
	default:
		r, g, b = fireBaseColors[0].r, fireBaseColors[0].g, fireBaseColors[0].b
	}

	// Wrong password animation: red shift (takes priority, uses timer)
	if s.wrongPasswordFrames > 0 {
		redIntensity := float64(s.wrongPasswordFrames) / float64(wrongPasswordDuration)
		r, g, b = fire.ApplyRedShift(r, g, b, redIntensity)
		return tcell.StyleDefault.Foreground(tcell.NewRGBColor(int32(r), int32(g), int32(b)))
	}

	// Color shift based on cell heat (same source as height).
	// After heat diffusion, values are lower than heatPower.
	if v > colorShiftBaseHeat {
		intensity := float64(v-colorShiftBaseHeat) / float64(colorShiftMaxHeat-colorShiftBaseHeat)
		if intensity > 1 {
			intensity = 1
		}
		r, g, b = fire.ApplyIntensityShift(r, g, b, intensity)
	}

	return tcell.StyleDefault.Foreground(tcell.NewRGBColor(int32(r), int32(g), int32(b)))
}

func (s *screensaver) renderTicker() {
	if !s.haveTicker || s.height < 2 || len(s.msgText) == 0 {
		return
	}

	msgRunes := []rune(s.msgText)
	metaRunes := []rune(s.metaText)
	msgRow := s.height - 2
	metaRow := s.height - 1
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite)

	for x := 0; x < s.width; x++ {
		mi := (s.tickerOffset + x) % len(msgRunes)
		mj := (s.tickerOffset + x) % len(metaRunes)
		s.screen.SetContent(x, msgRow, msgRunes[mi], nil, style)
		s.screen.SetContent(x, metaRow, metaRunes[mj], nil, style)
	}

	if s.frame%4 == 0 {
		s.tickerOffset = (s.tickerOffset + 1) % len(msgRunes)
	}
}

// ---- Command Execution

func execScreensaver(cfg screensaverConfig) error {
	if cfg.mode == ModeLock && !lock.PasswordExists() {
		return fmt.Errorf("no password configured. Run 'yule-log lock set-password' first")
	}

	s, err := newScreensaver(cfg)
	if err != nil {
		return err
	}
	defer s.close()

	return s.run()
}

type idleConfig struct {
	Timeout  int
	Once     bool
	Contribs bool
	NoTicker bool
}

func execIdle(cfg idleConfig) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}

	if cfg.Once {
		triggerScreensaver(context.Background(), exePath, triggerConfig{
			Contribs: cfg.Contribs,
			NoTicker: cfg.NoTicker,
		})
		return nil
	}

	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not running inside tmux")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Yule log idle watcher started (timeout: %ds, poll: %ds)\n", cfg.Timeout, pollInterval)

	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

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
				if idleSeconds < cfg.Timeout {
					waitingForActivity = false
				}
				continue
			}

			if idleSeconds >= cfg.Timeout {
				triggerScreensaver(ctx, exePath, triggerConfig{
					Contribs: cfg.Contribs,
					NoTicker: cfg.NoTicker,
				})
				waitingForActivity = true
			}
		}
	}
}

type lockConfig struct {
	SocketProtect bool
	Contribs      bool
	NoTicker      bool
	Cooldown      fire.CooldownSpeed
}

func execLock(cfg lockConfig) error {
	if !lock.PasswordExists() {
		return fmt.Errorf("no password configured. Run 'yule-log lock set-password' first")
	}

	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("not running inside tmux")
	}

	var socketPath string
	var originalPerm os.FileMode

	if cfg.SocketProtect {
		var err error
		socketPath, err = lock.GetTmuxSocketPath()
		if err != nil {
			return fmt.Errorf("getting tmux socket: %w", err)
		}

		originalPerm, err = lock.RestrictSocket(socketPath)
		if err != nil {
			return fmt.Errorf("restricting socket: %w", err)
		}
		defer lock.RestoreSocket(socketPath, originalPerm)
	}

	if err := lock.Lock(socketPath, originalPerm); err != nil {
		return fmt.Errorf("creating lock state: %w", err)
	}
	defer lock.Unlock()

	return execScreensaver(screensaverConfig{
		mode:     ModeLock,
		contribs: cfg.Contribs,
		noTicker: cfg.NoTicker,
		cooldown: cfg.Cooldown,
	})
}

func execSetPassword() error {
	reader := bufio.NewReader(os.Stdin)

	if lock.PasswordExists() {
		fmt.Print("A password is already set. Replace it? [y/N]: ")
		response, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Password not changed.")
			return nil
		}
	}

	fmt.Println("Set your lock password.")
	fmt.Println("You can use regular characters and arrow keys (shown as arrows).")
	fmt.Print("Enter password: ")

	password, err := readPasswordWithArrows()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	if len(password) == 0 {
		return fmt.Errorf("password cannot be empty")
	}
	defer lock.ClearBytes(password)

	fmt.Print("\nConfirm password: ")
	confirm, err := readPasswordWithArrows()
	if err != nil {
		return fmt.Errorf("reading confirmation: %w", err)
	}
	defer lock.ClearBytes(confirm)

	if subtle.ConstantTimeCompare(password, confirm) != 1 {
		return fmt.Errorf("passwords do not match")
	}

	if err := lock.SavePassword(password); err != nil {
		return fmt.Errorf("saving password: %w", err)
	}

	fmt.Println("\nPassword set successfully.")
	return nil
}

func execLockStatus() error {
	if lock.PasswordExists() {
		fmt.Println("Password: configured")
	} else {
		fmt.Println("Password: not configured")
	}

	if lock.IsLocked() {
		if duration, err := lock.LockDuration(); err == nil {
			fmt.Printf("Status: locked (for %s)\n", duration.Round(time.Second))
		} else {
			fmt.Println("Status: locked")
		}
	} else {
		fmt.Println("Status: unlocked")
	}

	return nil
}

// ---- Helpers

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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

	return max(int(time.Now().Unix()-activityTime), 0), nil
}

type triggerConfig struct {
	Contribs bool
	NoTicker bool
}

func triggerScreensaver(ctx context.Context, exePath string, cfg triggerConfig) {
	args := []string{exePath, "run"}
	if cfg.Contribs {
		args = append(args, "--contribs")
	}
	if cfg.NoTicker {
		args = append(args, "--no-ticker")
	}

	panePathCmd := exec.CommandContext(ctx, "tmux", "display-message", "-p", "#{pane_current_path}")
	if panePathOut, _ := panePathCmd.Output(); len(panePathOut) > 0 {
		if panePath := strings.TrimSpace(string(panePathOut)); panePath != "" {
			args = append(args, "--dir", panePath)
		}
	}

	cmd := exec.Command("tmux", "display-popup", "-E", "-w", "100%", "-h", "100%", strings.Join(args, " "))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Intentionally ignoring error: tmux display-popup may fail if:
	// - tmux server is unavailable
	// - running outside tmux
	// - popup already active
	// This is a best-effort trigger from the idle watcher, not critical.
	_ = cmd.Run()
}

// ---- Git Ticker

func buildGitTickerText(maxCommits int, gitDir string) (string, string, bool) {
	cmd := exec.Command("git", "log", "-n", strconv.Itoa(maxCommits), "--pretty=format:%h%x09%an%x09%ar%x09%s")

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

		author, relTime, subject := parts[1], parts[2], parts[3]
		meta := "by " + author + " " + relTime

		width := max(len([]rune(subject)), len([]rune(meta))) + 4
		msgSegs = append(msgSegs, padRight(subject, width))
		metaSegs = append(metaSegs, padRight(meta, width))
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

// ---- Password Input

// readPasswordWithArrows reads a password from stdin with arrow key support.
// Uses POSIX-secure terminal input via golang.org/x/term.
// Returns the password bytes or nil if cancelled (Escape or Ctrl+C).
func readPasswordWithArrows() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("stdin is not a terminal")
	}

	// Enter raw mode (disables echo and line buffering)
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("entering raw mode: %w", err)
	}

	// Ensure terminal is restored on exit
	defer term.Restore(fd, oldState)

	// Handle signals to restore terminal on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	doneChan := make(chan struct{})
	defer close(doneChan)

	go func() {
		select {
		case <-sigChan:
			term.Restore(fd, oldState)
			os.Exit(1)
		case <-doneChan:
		}
	}()

	var password []byte
	var displayLen int
	buf := make([]byte, 16)
	defer lock.ClearBytes(buf)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return password, nil
			}
			lock.ClearBytes(password)
			return nil, fmt.Errorf("reading input: %w", err)
		}

		for i := 0; i < n; {
			b := buf[i]

			switch {
			case b == '\r' || b == '\n': // Enter
				fmt.Print("\r\n")
				return password, nil

			case b == byteEscape: // Escape sequence
				if i+2 < n && buf[i+1] == '[' {
					// Arrow key: ESC [ A/B/C/D
					switch buf[i+2] {
					case 'A': // Up
						password = append(password, lock.ArrowUpMarker...)
						fmt.Print("\033[33m↑\033[0m") // Yellow arrow
						displayLen++
						i += 3
						continue
					case 'B': // Down
						password = append(password, lock.ArrowDownMarker...)
						fmt.Print("\033[33m↓\033[0m")
						displayLen++
						i += 3
						continue
					case 'C': // Right
						password = append(password, lock.ArrowRightMarker...)
						fmt.Print("\033[33m→\033[0m")
						displayLen++
						i += 3
						continue
					case 'D': // Left
						password = append(password, lock.ArrowLeftMarker...)
						fmt.Print("\033[33m←\033[0m")
						displayLen++
						i += 3
						continue
					}
				}
				// Plain Escape key - cancel
				fmt.Print("\r\n")
				lock.ClearBytes(password)
				return nil, nil

			case b == byteCtrlC:
				fmt.Print("\r\n")
				lock.ClearBytes(password)
				return nil, fmt.Errorf("interrupted")

			case b == byteBackspace || b == byteDelete:
				if len(password) > 0 && displayLen > 0 {
					password = handlePasswordBackspace(password)
					displayLen--
					// Erase last character from display
					fmt.Print("\b \b")
				}

			case b >= bytePrintableStart && b < bytePrintableEnd: // Printable ASCII
				password = append(password, b)
				fmt.Print("*")
				displayLen++

			default:
				// Ignore other control characters
			}

			i++
		}
	}
}

// handlePasswordBackspace removes the last character/marker from password.
func handlePasswordBackspace(password []byte) []byte {
	if len(password) == 0 {
		return password
	}

	// Handle multi-byte arrow markers
	if len(password) >= 2 {
		last2 := string(password[len(password)-2:])
		if last2 == lock.ArrowUpMarker || last2 == lock.ArrowDownMarker ||
			last2 == lock.ArrowLeftMarker || last2 == lock.ArrowRightMarker {
			return password[:len(password)-2]
		}
	}
	return password[:len(password)-1]
}

// ---- CLI Setup

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	rootCmd := buildCLI()
	return rootCmd.ParseAndRun(context.Background(), os.Args[1:])
}

func buildCLI() *ffcli.Command {
	// Run command
	runFlagSet := flag.NewFlagSet("yule-log run", flag.ExitOnError)
	runContribs := runFlagSet.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	runGitDir := runFlagSet.String("dir", "", "Git directory for commit ticker (defaults to current dir or YULE_LOG_GIT_DIR)")
	runNoTicker := runFlagSet.Bool("no-ticker", false, "Disable git commit ticker (fire animation only)")
	runPlayground := runFlagSet.Bool("playground", false, "Playground mode: only ESC exits, all keys affect fire")
	runCooldown := runFlagSet.String("cooldown", string(fire.DefaultCooldown), "Fire cooldown speed: fast, medium, slow")
	runLock := runFlagSet.Bool("lock", false, "Lock mode: require password to exit")

	runCmd := &ffcli.Command{
		Name:       "run",
		ShortUsage: "yule-log run [flags]",
		ShortHelp:  "Run the screensaver",
		FlagSet:    runFlagSet,
		Exec: func(_ context.Context, _ []string) error {
			mode := ModeNormal
			if *runLock {
				mode = ModeLock
			} else if *runPlayground {
				mode = ModePlayground
			}
			return execScreensaver(screensaverConfig{
				mode:     mode,
				contribs: *runContribs,
				gitDir:   *runGitDir,
				noTicker: *runNoTicker,
				cooldown: fire.CooldownSpeed(*runCooldown),
			})
		},
	}

	// Idle command
	idleFlagSet := flag.NewFlagSet("yule-log idle", flag.ExitOnError)
	idleTimeout := idleFlagSet.Int("timeout", defaultIdleTimeout, "Idle timeout in seconds before triggering screensaver")
	idleOnce := idleFlagSet.Bool("once", false, "Trigger screensaver immediately and exit")
	idleContribs := idleFlagSet.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	idleNoTicker := idleFlagSet.Bool("no-ticker", false, "Disable git commit ticker")

	idleCmd := &ffcli.Command{
		Name:       "idle",
		ShortUsage: "yule-log idle [flags]",
		ShortHelp:  "Run idle watcher daemon",
		FlagSet:    idleFlagSet,
		Exec: func(_ context.Context, _ []string) error {
			return execIdle(idleConfig{
				Timeout:  *idleTimeout,
				Once:     *idleOnce,
				Contribs: *idleContribs,
				NoTicker: *idleNoTicker,
			})
		},
	}

	// Lock command and subcommands
	lockFlagSet := flag.NewFlagSet("yule-log lock", flag.ExitOnError)
	lockSocketProtect := lockFlagSet.Bool("socket-protect", true, "Restrict tmux socket permissions during lock")
	lockContribs := lockFlagSet.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	lockNoTicker := lockFlagSet.Bool("no-ticker", false, "Disable git commit ticker")
	lockCooldown := lockFlagSet.String("cooldown", string(fire.DefaultCooldown), "Fire cooldown speed: fast, medium, slow")

	setPasswordCmd := &ffcli.Command{
		Name:       "set-password",
		ShortUsage: "yule-log lock set-password",
		ShortHelp:  "Set or update the lock password",
		Exec:       func(_ context.Context, _ []string) error { return execSetPassword() },
	}

	lockStatusCmd := &ffcli.Command{
		Name:       "status",
		ShortUsage: "yule-log lock status",
		ShortHelp:  "Show lock status",
		Exec:       func(_ context.Context, _ []string) error { return execLockStatus() },
	}

	lockCmd := &ffcli.Command{
		Name:        "lock",
		ShortUsage:  "yule-log lock [flags]",
		ShortHelp:   "Lock the tmux session",
		FlagSet:     lockFlagSet,
		Subcommands: []*ffcli.Command{setPasswordCmd, lockStatusCmd},
		Exec: func(_ context.Context, _ []string) error {
			return execLock(lockConfig{
				SocketProtect: *lockSocketProtect,
				Contribs:      *lockContribs,
				NoTicker:      *lockNoTicker,
				Cooldown:      fire.CooldownSpeed(*lockCooldown),
			})
		},
	}

	// Root command
	return &ffcli.Command{
		ShortUsage:  "yule-log [flags] <subcommand>",
		ShortHelp:   "A tmux screensaver with fire animation and git commit ticker",
		LongHelp:    "Controls:\n  Arrow Up/Down   Adjust flame intensity\n  Any other key   Exit screensaver\n\nLock mode:\n  All keys feed the fire, Enter submits password",
		FlagSet:     flag.NewFlagSet("yule-log", flag.ExitOnError),
		Subcommands: []*ffcli.Command{runCmd, idleCmd, lockCmd},
		Exec:        func(_ context.Context, _ []string) error { return execScreensaver(screensaverConfig{}) },
	}
}
