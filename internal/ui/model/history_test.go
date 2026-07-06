package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddToPromptHistorySkipsEmpty(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("")
	m.addToPromptHistory("   ")
	require.Empty(t, m.promptHistory.messages)
}

func TestAddToPromptHistoryPrepends(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("first")
	m.addToPromptHistory("second")
	m.addToPromptHistory("third")
	require.Equal(t, []string{"third", "second", "first"}, m.promptHistory.messages)
}

func TestAddToPromptHistoryDedupsConsecutive(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("hello")
	m.addToPromptHistory("hello")
	require.Len(t, m.promptHistory.messages, 1)
	require.Equal(t, "hello", m.promptHistory.messages[0])
}

func TestAddToPromptHistoryAllowsRepeatAfterDifferent(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("hello")
	m.addToPromptHistory("world")
	m.addToPromptHistory("hello")
	require.Equal(t, []string{"hello", "world", "hello"}, m.promptHistory.messages)
}

func TestAddToPromptHistoryPreservesExisting(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.promptHistory.messages = []string{"old1", "old2"}
	m.addToPromptHistory("new")
	require.Equal(t, []string{"new", "old1", "old2"}, m.promptHistory.messages)
}

func TestHistoryResetPreservesMessages(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("msg1")
	m.addToPromptHistory("msg2")
	m.promptHistory.index = 1
	m.promptHistory.draft = "draft text"
	m.historyReset()
	require.Equal(t, []string{"msg2", "msg1"}, m.promptHistory.messages)
	require.Equal(t, -1, m.promptHistory.index)
	require.Empty(t, m.promptHistory.draft)
}

func TestAddToPromptHistoryAfterReset(t *testing.T) {
	t.Parallel()
	m := &UI{}
	m.addToPromptHistory("msg1")
	m.historyReset()
	m.addToPromptHistory("msg2")
	require.Equal(t, []string{"msg2", "msg1"}, m.promptHistory.messages)
	require.Equal(t, -1, m.promptHistory.index)
}
