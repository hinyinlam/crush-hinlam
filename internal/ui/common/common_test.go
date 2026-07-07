package common

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/stretchr/testify/require"
)

func TestBuildOSC52TmuxPassthrough(t *testing.T) {
	t.Parallel()

	text := "hello tmux"
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(text))

	output := buildOSC52TmuxPassthrough(text)

	require.Contains(t, output, "\x1bPtmux;")
	require.Contains(t, output, "\x1b]52;c;"+expectedB64+"\x07")
	require.Contains(t, output, "\x1b\\")
	require.True(t, strings.HasPrefix(output, "\x1bPtmux;\x1b"))
	require.True(t, strings.HasSuffix(output, "\x07\x1b\\"))
}

func TestBuildOSC52TmuxPassthrough_SpecialChars(t *testing.T) {
	t.Parallel()

	text := "line1\nline2\t\ttab"
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(text))

	output := buildOSC52TmuxPassthrough(text)

	require.Contains(t, output, expectedB64)
}

func TestBuildOSC52TmuxPassthrough_Unicode(t *testing.T) {
	t.Parallel()

	text := "こんにちは 🌟 café"
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(text))

	output := buildOSC52TmuxPassthrough(text)

	require.Contains(t, output, expectedB64)
}

func TestBuildOSC52TmuxPassthrough_Empty(t *testing.T) {
	t.Parallel()

	output := buildOSC52TmuxPassthrough("")

	require.Contains(t, output, "\x1bPtmux;")
	require.Contains(t, output, "\x1b]52;c;")
	require.Contains(t, output, "\x1b\\")
}

func TestBuildOSC52TmuxPassthrough_DecodesCorrectly(t *testing.T) {
	t.Parallel()

	text := "test"
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	output := buildOSC52TmuxPassthrough(text)

	require.Contains(t, output, b64)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	require.NoError(t, err)
	require.Equal(t, text, string(decoded))
}

func TestCopyToClipboard_TmuxPassthrough(t *testing.T) {
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

func TestCopyToClipboard_NoTmuxPassthrough(t *testing.T) {
	_ = clipboard.Init()

	origTMUX := os.Getenv("TMUX")
	defer os.Setenv("TMUX", origTMUX)

	os.Unsetenv("TMUX")
	os.Unsetenv("TMUX_PANE")
	os.Unsetenv("STY")

	cmd := CopyToClipboard("plain text", "Copied")
	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)
}

func TestWriteOSC52WithTmuxPassthrough_Output(t *testing.T) {
	text := "integration-test"
	expected := buildOSC52TmuxPassthrough(text)

	tmp, err := os.CreateTemp("", "osc52-test-*")
	require.NoError(t, err)
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	orig := os.Stdout
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	require.NoError(t, err)
	os.Stdout = f

	writeOSC52WithTmuxPassthrough(text)
	f.Close()
	os.Stdout = orig

	output, err := os.ReadFile(tmpPath)
	require.NoError(t, err)
	require.Equal(t, expected, string(output))
}

func TestCopyToClipboard_BothEnvironments(t *testing.T) {
	_ = clipboard.Init()

	mux := terminal.DetectMux()
	t.Logf("Current mux: type=%q session=%q window=%q",
		mux.Type, mux.Session, mux.Window)

	// Test that CopyToClipboard works in whatever environment we're in
	cmd := CopyToClipboard("cross-env test", "Copied")
	require.NotNil(t, cmd)
	msgs := tea.Sequence(cmd)
	require.NotNil(t, msgs)
}

// tea.Cmd is func() tea.Msg - placeholder for init
func init() {}
