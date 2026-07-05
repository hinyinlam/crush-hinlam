package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGoalEvaluation_MetTrue(t *testing.T) {
	met, reason, err := parseGoalEvaluation("MET: true\nREASON: All tests pass and lint is clean.")
	require.NoError(t, err)
	require.True(t, met)
	require.Equal(t, "All tests pass and lint is clean.", reason)
}

func TestParseGoalEvaluation_MetFalse(t *testing.T) {
	met, reason, err := parseGoalEvaluation("MET: false\nREASON: 2 tests still failing.")
	require.NoError(t, err)
	require.False(t, met)
	require.Equal(t, "2 tests still failing.", reason)
}

func TestParseGoalEvaluation_CaseInsensitive(t *testing.T) {
	met, _, err := parseGoalEvaluation("met: TRUE\nreason: done")
	require.NoError(t, err)
	require.True(t, met)
}

func TestParseGoalEvaluation_YesValue(t *testing.T) {
	met, _, err := parseGoalEvaluation("MET: yes\nREASON: done")
	require.NoError(t, err)
	require.True(t, met)
}

func TestParseGoalEvaluation_NoReason(t *testing.T) {
	met, reason, err := parseGoalEvaluation("MET: true")
	require.NoError(t, err)
	require.True(t, met)
	require.Contains(t, reason, "satisfied")
}

func TestParseGoalEvaluation_NoReasonFalse(t *testing.T) {
	met, reason, err := parseGoalEvaluation("MET: false")
	require.NoError(t, err)
	require.False(t, met)
	require.Contains(t, reason, "not yet met")
}

func TestParseGoalEvaluation_UnrecognizedFormat(t *testing.T) {
	met, reason, err := parseGoalEvaluation("The goal seems to be mostly done but needs verification.")
	require.NoError(t, err)
	require.False(t, met)
	require.Contains(t, reason, "mostly done")
}

func TestParseGoalEvaluation_Empty(t *testing.T) {
	met, _, err := parseGoalEvaluation("")
	require.NoError(t, err)
	require.False(t, met)
}

func TestParseGoalEvaluation_WithThinkingTags(t *testing.T) {
	input := "<think>Let me analyze...</think>\nMET: true\nREASON: Tests pass."
	met, reason, err := parseGoalEvaluation(input)
	require.NoError(t, err)
	require.True(t, met)
	require.Equal(t, "Tests pass.", reason)
}
