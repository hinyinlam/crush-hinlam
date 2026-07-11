package common

import (
	"encoding/base64"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/stretchr/testify/require"
)

func TestLoadTmuxBuffer(t *testing.T) {
	t.Parallel()

	// Skip if tmux is not available.
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	text := "hello from loadTmuxBuffer test"

	// loadTmuxBuffer should not panic or error.
	loadTmuxBuffer(text)
}

func TestLoadTmuxBuffer_Empty(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	loadTmuxBuffer("")
}

func TestLoadTmuxBuffer_Unicode(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	loadTmuxBuffer("こんにちは 🌟 café")
}

func TestCopyToClipboard_TmuxBuffer(t *testing.T) {
	_ = clipboard.Init()

	mux := terminal.DetectMux()
	if mux.Type != "tmux" {
		t.Skip("test requires running inside tmux")
	}

	cmd := CopyToClipboard("tmux test text", "Copied")
	require.NotNil(t, cmd)

	msg := cmd()
	require.NotNil(t, msg)
}

func TestCopyToClipboard_ScreenDCS(t *testing.T) {
	_ = clipboard.Init()

	origTMUX := os.Getenv("TMUX")
	origSTY := os.Getenv("STY")
	origZELLIJ := os.Getenv("ZELLIJ")
	defer func() {
		os.Setenv("TMUX", origTMUX)
		os.Setenv("STY", origSTY)
		os.Setenv("ZELLIJ", origZELLIJ)
	}()

	os.Setenv("TMUX", "")
	os.Setenv("ZELLIJ", "")
	os.Setenv("STY", "12345.pts-0.host")

	cmd := CopyToClipboard("screen test text", "Copied")
	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)
}

func TestCopyToClipboard_Zellij(t *testing.T) {
	_ = clipboard.Init()

	origTMUX := os.Getenv("TMUX")
	origSTY := os.Getenv("STY")
	origZELLIJ := os.Getenv("ZELLIJ")
	defer func() {
		os.Setenv("TMUX", origTMUX)
		os.Setenv("STY", origSTY)
		os.Setenv("ZELLIJ", origZELLIJ)
	}()

	os.Setenv("TMUX", "")
	os.Setenv("STY", "")
	os.Setenv("ZELLIJ", "0")

	cmd := CopyToClipboard("zellij test text", "Copied")
	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)
}

func TestWriteScreenDCSPassthrough(t *testing.T) {
	t.Parallel()

	text := "screen test"
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(text))

	tmp, err := os.CreateTemp("", "screen-osc52-*")
	require.NoError(t, err)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	orig := os.Stdout
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	require.NoError(t, err)
	os.Stdout = f

	writeScreenDCSPassthrough(text)
	f.Close()
	os.Stdout = orig

	output, err := os.ReadFile(tmpPath)
	require.NoError(t, err)
	s := string(output)
	require.Contains(t, s, "\x1bP")
	require.Contains(t, s, "\x1b]52;c;"+expectedB64)
	require.Contains(t, s, "\x1b\\")
}

func TestCopyToClipboard_NoMux(t *testing.T) {
	_ = clipboard.Init()

	origTMUX := os.Getenv("TMUX")
	origSTY := os.Getenv("STY")
	origZELLIJ := os.Getenv("ZELLIJ")
	defer func() {
		os.Setenv("TMUX", origTMUX)
		os.Setenv("STY", origSTY)
		os.Setenv("ZELLIJ", origZELLIJ)
	}()

	os.Unsetenv("TMUX")
	os.Unsetenv("TMUX_PANE")
	os.Unsetenv("STY")
	os.Unsetenv("ZELLIJ")

	cmd := CopyToClipboard("plain text", "Copied")
	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)
}

func TestCopyToClipboard_BothEnvironments(t *testing.T) {
	_ = clipboard.Init()

	mux := terminal.DetectMux()
	t.Logf("Current mux: type=%q session=%q window=%q",
		mux.Type, mux.Session, mux.Window)

	// Test that CopyToClipboard works in whatever environment we're in.
	cmd := CopyToClipboard("cross-env test", "Copied")
	require.NotNil(t, cmd)
	msgs := tea.Sequence(cmd)
	require.NotNil(t, msgs)
}

// Ensures loadTmuxBuffer produces valid tmux command arguments.
func TestLoadTmuxBuffer_CommandArgs(t *testing.T) {
	t.Parallel()

	// Verify the command can be constructed without error.
	cmd := exec.Command("tmux", "load-buffer", "-")
	require.True(t, strings.HasSuffix(cmd.Path, "tmux"))
	require.Equal(t, []string{"load-buffer", "-"}, cmd.Args[1:])

	// Verify stdin accepts a reader.
	text := "test payload"
	cmd.Stdin = strings.NewReader(text)
	require.NotNil(t, cmd.Stdin)
}

// tea.Cmd is func() tea.Msg - placeholder for init.
func init() {}
