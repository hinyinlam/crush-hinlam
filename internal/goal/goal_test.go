package goal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetAndGet(t *testing.T) {
	s := NewService()
	st := s.Set("s1", "fix all tests")

	require.Equal(t, "fix all tests", st.Condition)
	require.True(t, st.Active)
	require.False(t, st.Paused)
	require.False(t, st.StartedAt.IsZero())

	got := s.Get("s1")
	require.NotNil(t, got)
	require.Equal(t, "fix all tests", got.Condition)
	require.True(t, got.Active)
}

func TestGetMissing(t *testing.T) {
	s := NewService()
	require.Nil(t, s.Get("nope"))
}

func TestClear(t *testing.T) {
	s := NewService()
	s.Set("s1", "do something")
	require.NotNil(t, s.Get("s1"))

	s.Clear("s1")
	require.Nil(t, s.Get("s1"))
}

func TestClearMissing(t *testing.T) {
	s := NewService()
	require.NotPanics(t, func() { s.Clear("nope") })
}

func TestPauseResume(t *testing.T) {
	s := NewService()
	s.Set("s1", "goal")
	require.False(t, s.Get("s1").Paused)

	s.Pause("s1")
	require.True(t, s.Get("s1").Paused)

	s.Resume("s1")
	require.False(t, s.Get("s1").Paused)
}

func TestPauseMissing(t *testing.T) {
	s := NewService()
	require.NotPanics(t, func() { s.Pause("nope") })
}

func TestIncrementTurn(t *testing.T) {
	s := NewService()
	s.Set("s1", "goal")

	require.Equal(t, 1, s.IncrementTurn("s1"))
	require.Equal(t, 2, s.IncrementTurn("s1"))
	require.Equal(t, 2, s.Get("s1").TurnCount)
}

func TestIncrementTurnMissing(t *testing.T) {
	s := NewService()
	require.Equal(t, 0, s.IncrementTurn("nope"))
}

func TestComplete(t *testing.T) {
	s := NewService()
	s.Set("s1", "goal")

	s.Complete("s1", "all tests pass")
	st := s.Get("s1")
	require.False(t, st.Active)
	require.False(t, st.CompletedAt.IsZero())
	require.Equal(t, "all tests pass", st.LastReason)
}

func TestSetReason(t *testing.T) {
	s := NewService()
	s.Set("s1", "goal")

	s.SetReason("s1", "still working")
	require.Equal(t, "still working", s.Get("s1").LastReason)
}

func TestReplaceGoal(t *testing.T) {
	s := NewService()
	s.Set("s1", "first goal")
	time.Sleep(time.Millisecond)
	s.Set("s1", "second goal")

	st := s.Get("s1")
	require.Equal(t, "second goal", st.Condition)
	require.True(t, st.Active)
}

func TestConcurrentAccess(t *testing.T) {
	s := NewService()
	done := make(chan struct{})

	go func() {
		for i := range 100 {
			s.Set("s1", "goal")
			s.IncrementTurn("s1")
			s.Get("s1")
			if i%10 == 0 {
				s.Clear("s1")
			}
		}
		done <- struct{}{}
	}()

	go func() {
		for range 100 {
			s.Set("s2", "goal2")
			s.Pause("s2")
			s.Resume("s2")
		}
		done <- struct{}{}
	}()

	<-done
	<-done
}
