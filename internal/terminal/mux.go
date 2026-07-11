// Package terminal provides terminal-related utilities including
// multiplexer (tmux, screen, zellij) detection.
package terminal

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// MuxInfo holds information about a detected terminal multiplexer.
type MuxInfo struct {
	// Type is the multiplexer type ("tmux", "screen", "zellij", or "" if none).
	Type string
	// Session is the session name (e.g. "0" for tmux, "12345.pts-0" for screen).
	Session string
	// Window is the window name within the session, if available.
	Window string
	// EnvWasSet is true when the info came from an environment variable
	// (fast path); false when detected via process tree traversal.
	EnvWasSet bool
}

// Display returns a human-readable label for the multiplexer info,
// suitable for compact display in a status bar or header.
func (m MuxInfo) Display() string {
	if m.Type == "" {
		return ""
	}
	s := m.Type + ":" + m.Session
	if m.Window != "" {
		s += "@" + m.Window
	}
	if !m.EnvWasSet {
		s += "*"
	}
	return s
}

// DetectMux detects whether the current process is running inside a
// terminal multiplexer (tmux, screen, or zellij). It first checks
// environment variables ($TMUX, $STY, $ZELLIJ), then falls back to
// traversing the process tree via /proc.
func DetectMux() MuxInfo {
	if info, ok := detectFromEnv(); ok {
		return info
	}
	return detectFromProc()
}

// detectFromEnv checks the TMUX, STY, and ZELLIJ environment variables.
// These can be unset when privilege escalation (sudo, su) clears the
// environment.
func detectFromEnv() (MuxInfo, bool) {
	if tmux := os.Getenv("TMUX"); tmux != "" {
		info := MuxInfo{
			Type:      "tmux",
			Session:   parseTmuxSession(tmux),
			EnvWasSet: true,
		}
		if pane := os.Getenv("TMUX_PANE"); pane != "" {
			if w := parseTmuxWindow(pane); w != "" {
				info.Window = w
			}
		}
		if info.Window == "" {
			info.Window = resolveTmuxWindowNameFromEnv()
		}
		return info, true
	}
	if sty := os.Getenv("STY"); sty != "" {
		return MuxInfo{
			Type:      "screen",
			Session:   sty,
			EnvWasSet: true,
		}, true
	}
	if zellij := os.Getenv("ZELLIJ"); zellij != "" {
		info := MuxInfo{
			Type:      "zellij",
			EnvWasSet: true,
		}
		info.Session = os.Getenv("ZELLIJ_SESSION_NAME")
		return info, true
	}
	return MuxInfo{}, false
}

// detectFromProc walks the process tree upward from the current PID,
// checking each ancestor's comm name for tmux, screen, or zellij.
func detectFromProc() MuxInfo {
	pid := os.Getpid()
	for depth := 0; depth < 64 && pid > 1; depth++ {
		comm, err := readProcComm(pid)
		if err != nil {
			return MuxInfo{}
		}
		switch {
		case strings.HasPrefix(comm, "tmux"):
			info := MuxInfo{
				Type: "tmux",
			}
			info.Session = resolveFromProcEnv("TMUX", parseTmuxSession)
			if pane := resolveFromProcEnv("TMUX_PANE", identity); pane != "" {
				if w := parseTmuxWindow(pane); w != "" {
					info.Window = w
				}
			}
			if info.Window == "" {
				info.Window = resolveTmuxWindowName()
			}
			return info
		case comm == "screen":
			return MuxInfo{
				Type:    "screen",
				Session: resolveFromProcEnv("STY", identity),
			}
		case strings.HasPrefix(comm, "zellij"):
			return MuxInfo{
				Type:    "zellij",
				Session: resolveFromProcEnv("ZELLIJ_SESSION_NAME", identity),
			}
		}
		ppid := readProcPPID(pid)
		if ppid <= 0 || ppid >= pid {
			return MuxInfo{}
		}
		pid = ppid
	}
	return MuxInfo{}
}

// identity returns s unchanged; used as a no-op transform for resolveFromProcEnv.
func identity(s string) string { return s }

// resolveTmuxWindowName attempts to get the current tmux window name by
// reading $TMUX from a parent process's environ to find the socket path,
// then running tmux display-message.
func resolveTmuxWindowName() string {
	tmuxEnv := resolveFromProcEnv("TMUX", identity)
	if tmuxEnv == "" {
		return ""
	}
	return tmuxWindowNameFromSocket(tmuxEnv)
}

// resolveTmuxWindowNameFromEnv resolves the window name using the current
// process's $TMUX environment variable.
func resolveTmuxWindowNameFromEnv() string {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return ""
	}
	return tmuxWindowNameFromSocket(tmuxEnv)
}

// tmuxWindowNameFromSocket takes a $TMUX value and queries tmux for the
// window name via the socket path embedded in it.
func tmuxWindowNameFromSocket(tmuxEnv string) string {
	// TMUX format: /path/socket,server_pid,session_id
	comma := strings.IndexByte(tmuxEnv, ',')
	if comma < 0 {
		return ""
	}
	socket := tmuxEnv[:comma]

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "-S", socket, "display-message", "-p", "#{window_name}")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveFromProcEnv walks up from the current process and reads each
// ancestor's /proc/{pid}/environ to find the given environment variable,
// then applies the transform function to the value.
func resolveFromProcEnv(key string, transform func(string) string) string {
	pid := os.Getpid()
	for depth := 0; depth < 64 && pid > 1; depth++ {
		if val := readProcEnvVar(pid, key); val != "" {
			return transform(val)
		}
		ppid := readProcPPID(pid)
		if ppid <= 0 || ppid >= pid {
			break
		}
		pid = ppid
	}
	return ""
}

// parseTmuxSession extracts the session name from a TMUX environment
// variable value like "/tmp/tmux-1000/default,1234,0".
func parseTmuxSession(tmux string) string {
	idx := strings.LastIndexByte(tmux, ',')
	if idx < 0 {
		return tmux
	}
	return tmux[idx+1:]
}

// parseTmuxPane extracts the window name from a TMUX_PANE value like
// "/tmp/tmux-1000/default,6789,0:1.0". Returns the session:window portion
// (e.g. "0:1").
func parseTmuxWindow(pane string) string {
	// TMUX_PANE format: /path/socket,pane_pid,session:window.pane_index
	idx := strings.LastIndexByte(pane, ',')
	if idx < 0 {
		return ""
	}
	rest := pane[idx+1:] // e.g. "0:1.0" or "6789" (just pane PID, no window info)
	colonIdx := strings.IndexByte(rest, ':')
	if colonIdx < 0 {
		return "" // no window info in this PANE value
	}
	afterColon := rest[colonIdx+1:] // e.g. "1.0"
	dotIdx := strings.IndexByte(afterColon, '.')
	if dotIdx >= 0 {
		return rest[:colonIdx+1+dotIdx] // e.g. "0:1"
	}
	return rest
}

// readProcEnvVar reads a specific environment variable from
// /proc/{pid}/environ. Returns empty string if not found or unreadable.
func readProcEnvVar(pid int, key string) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, entry := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

// readProcComm reads /proc/{pid}/comm and returns the trimmed contents.
func readProcComm(pid int) (string, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readProcPPID reads the PPid field from /proc/{pid}/status.
func readProcPPID(pid int) int {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, err := strconv.Atoi(fields[1])
				if err != nil {
					return -1
				}
				return ppid
			}
		}
	}
	return -1
}
