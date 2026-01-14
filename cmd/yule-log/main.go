package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// parseGitLogToTicker converts `git log` output into two long strings:
// one for commit messages and one for "by AUTHOR REL_TIME" meta lines.
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
		// Fixed card width so message/meta line up as columns.
		// Use rune counts so multi-byte characters don't break alignment.
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

// buildGitTickerText runs git log and returns the scrolling texts.
// If gitDir is non-empty, it's used as the working directory; otherwise
// YULE_LOG_GIT_DIR env var or current directory is used.
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

func main() {
	// Parse command-line flags.
	contribs := flag.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	gitDir := flag.String("dir", "", "Git directory for commit ticker (defaults to current dir or YULE_LOG_GIT_DIR)")
	noTicker := flag.Bool("no-ticker", false, "Disable git commit ticker (fire animation only)")
	flag.Parse()

	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating screen: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "initializing screen: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()

	s.Clear()
	s.HideCursor()

	width, height := s.Size()
	if width <= 0 || height <= 0 {
		return
	}

	size := width * height
	buffer := make([]int, size+width+1)

	var chars []rune
	var styles []tcell.Style

	if *contribs {
		// GitHub contribution graph-style glyphs and colors.
		// Use varied glyphs for different intensity levels (10 total to match original behavior)
		chars = []rune{' ', '⬝', '⬝', '⯀', '⯀', '◼', '◼', '■', '■', '■'}
		// GitHub green palette: empty, low (#9be9a8), medium (#40c463), high (#30a14e), very high (#216e39)
		styles = []tcell.Style{
			tcell.StyleDefault.Foreground(tcell.ColorBlack),                 // index 0 (not used by style selection)
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(155, 233, 168)), // #9be9a8 (low)
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(64, 196, 99)),   // #40c463 (medium)
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(48, 161, 78)),   // #30a14e (high)
			tcell.StyleDefault.Foreground(tcell.NewRGBColor(33, 110, 57)),   // #216e39 (very high)
		}
	} else {
		// Original fire-style glyphs and colors.
		chars = []rune{' ', '.', ':', '^', '*', 'x', 's', 'S', '#', '$'}
		// Colors: dark red -> bright yellow/white.
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
	// Fire simulation constants.
	const (
		maxTickerCommits  = 20 // number of git commits to show in ticker
		defaultHeatPower  = 75 // initial flame intensity
		heatSourceDivisor = 6  // width / this = number of heat injection points
		minHeat           = 10
		maxHeat           = 85
		minSources        = 1
	)
	heatPower := defaultHeatPower
	heatSources := width / heatSourceDivisor
	if !*noTicker {
		msgText, metaText, haveTicker = buildGitTickerText(maxTickerCommits, *gitDir)
	}

	// Event polling goroutine: terminates when s.Fini() causes PollEvent to return nil.
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
		// Non-blocking input check.
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			// Handle arrow key presses to adjust heat scaling.
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
					break loop // any other key exits
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

		// Inject heat on bottom row to be scaled by arrow keys.
		for i := 0; i < heatSources; i++ {
			idx := rand.Intn(width) + width*(height-1)
			if idx >= 0 && idx < len(buffer) {
				buffer[idx] = heatPower
			}
		}
		// Propagate and cool.
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
			// Reserve bottom two lines for git info if available.
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

		// Draw git info as two aligned lines at bottom.
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
}
