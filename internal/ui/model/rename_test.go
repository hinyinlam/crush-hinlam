package model

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// renameTestWorkspace embeds testWorkspace and overrides methods needed
// by handleRenameCommand.
type renameTestWorkspace struct {
	testWorkspace
	agentReady bool
	renameErr  error
	renamedID  string
	renamedTo  string
}

func (w *renameTestWorkspace) AgentIsReady() bool { return w.agentReady }
func (w *renameTestWorkspace) SaveSession(_ context.Context, s session.Session) (session.Session, error) {
	w.renamedID = s.ID
	w.renamedTo = s.Title
	if w.renameErr != nil {
		return session.Session{}, w.renameErr
	}
	return s, nil
}

func newTestUIForRename(t *testing.T, agentReady bool) *UI {
	t.Helper()
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{},
	}
	w := &renameTestWorkspace{
		testWorkspace: testWorkspace{cfg: cfg},
		agentReady:    agentReady,
	}
	com := common.DefaultCommon(w)
	return &UI{
		com: com,
		session: &session.Session{
			ID:    "test-session-id",
			Title: "My Project",
		},
	}
}

func workspaceFromUI(u *UI) *renameTestWorkspace {
	return u.com.Workspace.(*renameTestWorkspace)
}

func TestHandleRenameCommandShowsCurrentName(t *testing.T) {
	t.Parallel()
	u := newTestUIForRename(t, true)
	cmd := u.handleRenameCommand("/rename")
	require.NotNil(t, cmd)
}

func TestHandleRenameCommandRenames(t *testing.T) {
	t.Parallel()
	u := newTestUIForRename(t, true)
	cmd := u.handleRenameCommand("/rename New Project Name")
	require.NotNil(t, cmd)
	// Execute the command to trigger the rename.
	cmd()
	w := workspaceFromUI(u)
	require.Equal(t, "test-session-id", w.renamedID)
	require.Equal(t, "New Project Name", w.renamedTo)
	require.Equal(t, "New Project Name", u.session.Title)
}

func TestHandleRenameCommandNoSession(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{},
	}
	w := &renameTestWorkspace{
		testWorkspace: testWorkspace{cfg: cfg},
		agentReady:    true,
	}
	u := &UI{
		com: &common.Common{
			Workspace: w,
		},
	}
	cmd := u.handleRenameCommand("/rename")
	require.NotNil(t, cmd)
}

func TestHandleRenameCommandAgentNotReady(t *testing.T) {
	t.Parallel()
	u := newTestUIForRename(t, false)
	cmd := u.handleRenameCommand("/rename New Name")
	require.NotNil(t, cmd)
}
