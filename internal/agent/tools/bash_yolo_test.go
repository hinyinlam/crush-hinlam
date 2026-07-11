package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

func TestBlockFuncsYolo(t *testing.T) {
	if bf := blockFuncs(true); bf != nil {
		t.Fatal("blockFuncs(true) should return nil")
	}
	if bf := blockFuncs(false); len(bf) == 0 {
		t.Fatal("blockFuncs(false) should return blocked commands")
	}
}

func TestBashToolYoloToggle(t *testing.T) {
	ps := permission.NewPermissionService("/tmp", false, nil)
	if ps.SkipRequests() {
		t.Fatal("expected SkipRequests=false")
	}
	ps.SetSkipRequests(true)
	if !ps.SkipRequests() {
		t.Fatal("expected SkipRequests=true after toggle")
	}
	ps.SetSkipRequests(false)
	if ps.SkipRequests() {
		t.Fatal("expected SkipRequests=false after second toggle")
	}
}

func TestBashToolCreationYolo(t *testing.T) {
	attr := &config.Attribution{}
	psNoYolo := permission.NewPermissionService("/tmp", false, nil)
	if tool := NewBashTool(psNoYolo, "/tmp", attr, "test-model", 0, 0); tool == nil {
		t.Fatal("NewBashTool returned nil for non-yolo")
	}
	psYolo := permission.NewPermissionService("/tmp", true, nil)
	if tool := NewBashTool(psYolo, "/tmp", attr, "test-model", 0, 0); tool == nil {
		t.Fatal("NewBashTool returned nil for yolo")
	}
}
