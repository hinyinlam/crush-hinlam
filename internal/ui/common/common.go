package common

import (
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/terminal"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
)

// MaxAttachmentSize defines the maximum allowed size for file attachments (5 MB).
const MaxAttachmentSize = int64(5 * 1024 * 1024)

// AllowedImageTypes defines the permitted image file types.
var AllowedImageTypes = []string{".jpg", ".jpeg", ".png"}

// Common defines common UI options and configurations.
type Common struct {
	Workspace workspace.Workspace
	Styles    *styles.Styles
}

// Config returns the pure-data configuration associated with this [Common] instance.
func (c *Common) Config() *config.Config {
	return c.Workspace.Config()
}

// DefaultCommon returns the default common UI configurations. When the
// workspace has a large model selected, the theme is chosen based on its
// provider; otherwise the default theme is used.
func DefaultCommon(ws workspace.Workspace) *Common {
	s := styles.ThemeForProvider(largeModelProviderID(ws))
	return &Common{
		Workspace: ws,
		Styles:    &s,
	}
}

// largeModelProviderID returns the provider ID of the currently selected
// large model, or the empty string if none is set or the workspace is nil.
func largeModelProviderID(ws workspace.Workspace) string {
	if ws == nil {
		return ""
	}
	cfg := ws.Config()
	if cfg == nil {
		return ""
	}
	return cfg.Models[config.SelectedModelTypeLarge].Provider
}

// IsHyper reports whether the currently selected large model is provided
// by Hyper.
func (c *Common) IsHyper() bool {
	return largeModelProviderID(c.Workspace) == "hyper"
}

// CenterRect returns a new [Rectangle] centered within the given area with the
// specified width and height.
func CenterRect(area uv.Rectangle, width, height int) uv.Rectangle {
	centerX := area.Min.X + area.Dx()/2
	centerY := area.Min.Y + area.Dy()/2
	minX := centerX - width/2
	minY := centerY - height/2
	maxX := minX + width
	maxY := minY + height
	return image.Rect(minX, minY, maxX, maxY)
}

// BottomLeftRect returns a new [Rectangle] positioned at the bottom-left within the given area with the
// specified width and height.
func BottomLeftRect(area uv.Rectangle, width, height int) uv.Rectangle {
	minX := area.Min.X
	maxX := minX + width
	maxY := area.Max.Y
	minY := maxY - height
	return image.Rect(minX, minY, maxX, maxY)
}

// IsFileTooBig checks if the file at the given path exceeds the specified size
// limit.
func IsFileTooBig(filePath string, sizeLimit int64) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("error getting file info: %w", err)
	}

	if fileInfo.Size() > sizeLimit {
		return true, nil
	}

	return false, nil
}

// CopyToClipboard copies the given text to the clipboard using both OSC 52
// (terminal escape sequence) and native clipboard for maximum compatibility.
// Returns a command that reports success to the user with the given message.
func CopyToClipboard(text, successMessage string) tea.Cmd {
	return CopyToClipboardWithCallback(text, successMessage, nil)
}

// CopyToClipboardWithCallback copies text to clipboard and executes a callback
// before showing the success message.
// This is useful when you need to perform additional actions like clearing UI state.
func CopyToClipboardWithCallback(text, successMessage string, callback tea.Cmd) tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetClipboard(text),
		func() tea.Msg {
			clipboard.WriteText(text)
			return nil
		},
		callback,
	}
	// Report honest status: when the native clipboard is available the copy
	// definitely worked (OSC 52 + native). When it's not available we rely
	// solely on OSC 52 which depends on terminal support.
	if clipboard.Available() {
		cmds = append(cmds, util.ReportInfo(successMessage))
	} else {
		cmds = append(cmds, util.ReportInfo(successMessage+" (via terminal OSC 52)"))
	}
	// Each multiplexer intercepts or drops OSC 52 from stdout differently.
	// Use the mux-specific workaround to ensure the clipboard reaches the
	// outer terminal.
	switch mux := terminal.DetectMux(); mux.Type {
	case "tmux":
		cmds = append(cmds, func() tea.Msg {
			loadTmuxBuffer(text)
			return nil
		})
	case "screen":
		cmds = append(cmds, func() tea.Msg {
			writeScreenDCSPassthrough(text)
			return nil
		})
	case "zellij":
		cmds = append(cmds, func() tea.Msg {
			writeOSC52ToTTY(text)
			return nil
		})
	}
	return tea.Sequence(cmds...)
}

// loadTmuxBuffer pipes text into `tmux load-buffer -`, setting tmux's
// internal paste buffer. When set-clipboard is on or external (the default),
// tmux then forwards the buffer contents to the outer terminal via OSC 52.
// This works without requiring allow-passthrough, unlike DCS passthrough
// escape sequences which tmux silently strips when allow-passthrough is off.
func loadTmuxBuffer(text string) {
	cmd := exec.Command("tmux", "load-buffer", "-")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

// writeScreenDCSPassthrough writes an OSC 52 clipboard sequence wrapped in
// GNU screen's DCS passthrough to stdout. Unlike tmux, screen passes DCS
// passthrough through to the outer terminal by default — no equivalent of
// tmux's allow-passthrough setting exists. Screen does not natively recognize
// OSC 52, so the DCS wrapper is required for the outer terminal to see it.
func writeScreenDCSPassthrough(text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	// DCS passthrough: ESC P <escaped OSC 52> ESC \
	// The OSC 52 inside is doubled (ESC ESC) so screen un-escapes it correctly.
	os.Stdout.WriteString("\x1bP\x1b\x1b]52;c;" + b64 + "\x07\x1b\\")
}

// writeOSC52ToTTY writes a plain OSC 52 sequence directly to /dev/tty,
// bypassing Zellij's interception of stdout OSC 52 sequences. Zellij
// intercepts OSC 52 emitted on the PTY (stdout) but does not intercept
// writes to the underlying terminal device. This requires the outer
// terminal to support OSC 52.
func writeOSC52ToTTY(text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("\x1b]52;c;" + b64 + "\x07")
}
