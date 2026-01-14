package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParseGitLogToTicker_NoOutput(t *testing.T) {
	if _, _, ok := parseGitLogToTicker(""); ok {
		t.Fatalf("expected ok=false for empty log")
	}
}

func TestParseGitLogToTicker_SingleCommit(t *testing.T) {
	log := "abcd1234\tAlice\t3 days ago\tInitial commit"
	msg, meta, ok := parseGitLogToTicker(log)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if msg == "" || meta == "" {
		t.Fatalf("expected non-empty message and meta")
	}
	if got, want := msg, "Initial commit"; !contains(got, want) {
		t.Fatalf("message %q does not contain %q", got, want)
	}
	if got, want := meta, "by Alice 3 days ago"; !contains(got, want) {
		t.Fatalf("meta %q does not contain %q", got, want)
	}
}

func TestParseGitLogToTicker_MultipleCommits(t *testing.T) {
	log := "" +
		"abcd1234\tAlice\t3 days ago\tInitial commit\n" +
		"efgh5678\tBob\t2 weeks ago\tAdd feature X\n" +
		"ijkl9012\tCarol\t1 year ago\tRefactor module Y\n"
	msg, meta, ok := parseGitLogToTicker(log)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	for _, want := range []string{"Initial commit", "Add feature X", "Refactor module Y"} {
		if !contains(msg, want) {
			t.Fatalf("message ticker %q does not contain %q", msg, want)
		}
	}
	for _, want := range []string{"by Alice 3 days ago", "by Bob 2 weeks ago", "by Carol 1 year ago"} {
		if !contains(meta, want) {
			t.Fatalf("meta ticker %q does not contain %q", meta, want)
		}
	}
}

func TestFlagParsing(t *testing.T) {
	// Save original command-line args and restore after test.
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		// Reset flag package state for subsequent tests
		flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ExitOnError)
	}()

	// Test with no flags (default behavior)
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	contribs := flag.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if *contribs {
		t.Fatalf("expected contribs to be false by default")
	}

	// Test with --contribs flag
	os.Args = []string{"cmd", "--contribs"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	contribs = flag.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		t.Fatalf("failed to parse --contribs: %v", err)
	}
	if !*contribs {
		t.Fatalf("expected contribs to be true with --contribs flag")
	}

	// Test with -contribs flag
	os.Args = []string{"cmd", "-contribs"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	contribs = flag.Bool("contribs", false, "Use GitHub contribution graph-style visualization")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		t.Fatalf("failed to parse -contribs: %v", err)
	}
	if !*contribs {
		t.Fatalf("expected contribs to be true with -contribs flag")
	}
}

func TestNewFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = flag.NewFlagSet(oldArgs[0], flag.ExitOnError)
	}()

	// Test --dir flag
	os.Args = []string{"cmd", "--dir", "/some/path"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	gitDir := flag.String("dir", "", "Git directory")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		t.Fatalf("failed to parse --dir: %v", err)
	}
	if *gitDir != "/some/path" {
		t.Fatalf("expected gitDir to be /some/path, got %q", *gitDir)
	}

	// Test --no-ticker flag
	os.Args = []string{"cmd", "--no-ticker"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	noTicker := flag.Bool("no-ticker", false, "Disable ticker")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		t.Fatalf("failed to parse --no-ticker: %v", err)
	}
	if !*noTicker {
		t.Fatalf("expected noTicker to be true with --no-ticker flag")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
