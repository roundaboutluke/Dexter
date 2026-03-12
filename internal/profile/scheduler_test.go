package profile

import "testing"

func TestSchedulerRefreshAlertStateInvokesCallback(t *testing.T) {
	s := &Scheduler{}
	called := 0
	s.SetRefreshAlertState(func() {
		called++
	})

	s.refreshAlertState()

	if called != 1 {
		t.Fatalf("refresh callback count=%d, want 1", called)
	}
}

func TestSchedulerRefreshAlertStateWithoutCallback(t *testing.T) {
	s := &Scheduler{}
	s.refreshAlertState()
}
