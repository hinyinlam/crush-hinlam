package dialog

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// stubWorkspace satisfies workspace.Workspace with only Config() returning
// a real value; all other methods get nil/zero from the embedded nil.
type stubWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (s *stubWorkspace) Config() *config.Config { return s.cfg }

func testCommonWithConfig() *common.Common {
	s := styles.ThemeForProvider("")
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {ID: "coder"},
		},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	return &common.Common{
		Styles:    &s,
		Workspace: &stubWorkspace{cfg: cfg},
	}
}

func TestDefaultCommands_GoalEntriesWithSession(t *testing.T) {
	c := &Commands{
		com:        testCommonWithConfig(),
		hasSession: true,
	}

	items := c.defaultCommands()

	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID())
	}

	require.Contains(t, ids, "set_goal")
	require.Contains(t, ids, "goal_status")
	require.Contains(t, ids, "clear_goal")
}

func TestDefaultCommands_GoalEntriesHiddenWithoutSession(t *testing.T) {
	c := &Commands{
		com:        testCommonWithConfig(),
		hasSession: false,
	}

	items := c.defaultCommands()

	for _, item := range items {
		require.NotEqual(t, "goal_status", item.ID())
		require.NotEqual(t, "clear_goal", item.ID())
	}
}

func TestDefaultCommands_GoalActions(t *testing.T) {
	c := &Commands{
		com:        testCommonWithConfig(),
		hasSession: true,
	}

	items := c.defaultCommands()

	for _, item := range items {
		switch item.ID() {
		case "set_goal":
			require.IsType(t, ActionSetGoal{}, item.Action())
		case "clear_goal":
			require.IsType(t, ActionClearGoal{}, item.Action())
		case "goal_status":
			require.IsType(t, ActionGoalStatus{}, item.Action())
		}
	}
}

var _ = context.Background
