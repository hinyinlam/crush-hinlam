package terminal

import (
	"os"
	"testing"
)

func TestDetectFromEnv_Tmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	t.Setenv("TMUX_PANE", "/tmp/tmux-1000/default,6789,0:1.0")
	t.Setenv("STY", "")
	info, ok := detectFromEnv()
	if !ok {
		t.Fatal("expected detection from TMUX env")
	}
	if info.Type != "tmux" {
		t.Fatalf("expected type tmux, got %q", info.Type)
	}
	if info.Session != "0" {
		t.Fatalf("expected session 0, got %q", info.Session)
	}
	if info.Window != "0:1" {
		t.Fatalf("expected window 0:1, got %q", info.Window)
	}
	if !info.EnvWasSet {
		t.Fatal("expected EnvWasSet to be true")
	}
}

func TestDetectFromEnv_Screen(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "12345.pts-0.host")
	info, ok := detectFromEnv()
	if !ok {
		t.Fatal("expected detection from STY env")
	}
	if info.Type != "screen" {
		t.Fatalf("expected type screen, got %q", info.Type)
	}
	if info.Session != "12345.pts-0.host" {
		t.Fatalf("expected session name 12345.pts-0.host, got %q", info.Session)
	}
	if !info.EnvWasSet {
		t.Fatal("expected EnvWasSet to be true")
	}
}

func TestDetectFromEnv_None(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
	info, ok := detectFromEnv()
	if ok {
		t.Fatal("expected no detection with empty env vars")
	}
	if info.Type != "" {
		t.Fatalf("expected empty type, got %q", info.Type)
	}
}

func TestDetectMux_EnvPriority(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
	info := DetectMux()
	if info.Type != "tmux" {
		t.Fatalf("expected tmux, got %q", info.Type)
	}
}

func TestParseTmuxSession(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"/tmp/tmux-1000/default,1234,0", "0"},
		{"/tmp/tmux-1000/default,1234,mysession", "mysession"},
		{"noseparator", "noseparator"},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseTmuxSession(tt.input)
		if got != tt.expected {
			t.Errorf("parseTmuxSession(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseTmuxWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"/tmp/tmux-1000/default,6789,0:1.0", "0:1"},
		{"/tmp/tmux-1000/default,6789,mysession:3.2", "mysession:3"},
		{"noseparator", ""},
		{"", ""},
		{"/tmp/tmux-1000/default,6789", ""},
	}
	for _, tt := range tests {
		got := parseTmuxWindow(tt.input)
		if got != tt.expected {
			t.Errorf("parseTmuxWindow(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMuxInfoDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		info     MuxInfo
		expected string
	}{
		{MuxInfo{}, ""},
		{MuxInfo{Type: "tmux", Session: "0", EnvWasSet: true}, "tmux:0"},
		{MuxInfo{Type: "tmux", Session: "0", Window: "0:1", EnvWasSet: true}, "tmux:0@0:1"},
		{MuxInfo{Type: "tmux", Session: "0", Window: "0:1", EnvWasSet: false}, "tmux:0@0:1*"},
		{MuxInfo{Type: "screen", Session: "12345.pts-0", EnvWasSet: true}, "screen:12345.pts-0"},
	}
	for _, tt := range tests {
		got := tt.info.Display()
		if got != tt.expected {
			t.Errorf("MuxInfo.Display() = %q, want %q", got, tt.expected)
		}
	}
}

func TestDetectMux_FromProc_InTmux(t *testing.T) {
	os.Unsetenv("TMUX")
	os.Unsetenv("STY")
	info := detectFromProc()
	if info.Type == "" {
		t.Skip("not running inside tmux/screen")
	}
	if info.Type != "tmux" && info.Type != "screen" {
		t.Fatalf("unexpected type: %q", info.Type)
	}
}

func TestReadProcComm_Init(t *testing.T) {
	comm, err := readProcComm(1)
	if err != nil {
		t.Skipf("cannot read /proc/1/comm: %v", err)
	}
	if comm == "" {
		t.Fatal("expected non-empty comm for PID 1")
	}
}

func TestReadProcPPID_Init(t *testing.T) {
	ppid := readProcPPID(1)
	if ppid != 0 {
		return
	}
}

func TestReadProcPPID_Current(t *testing.T) {
	pid := os.Getpid()
	ppid := readProcPPID(pid)
	if ppid <= 1 {
		t.Fatalf("expected PPID > 1 for PID %d, got %d", pid, ppid)
	}
}

func TestReadProcComm_Current(t *testing.T) {
	pid := os.Getpid()
	comm, err := readProcComm(pid)
	if err != nil {
		t.Fatalf("readProcComm(%d) error: %v", pid, err)
	}
	if comm == "" {
		t.Fatal("expected non-empty comm for current PID")
	}
}
