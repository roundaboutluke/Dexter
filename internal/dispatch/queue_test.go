package dispatch

import "testing"

func TestQueue_Empty(t *testing.T) {
	q := NewQueue("test")
	if q.Len() != 0 {
		t.Errorf("Len() = %d, want 0", q.Len())
	}
	jobs := q.Drain()
	if jobs != nil {
		t.Errorf("Drain() = %v, want nil", jobs)
	}
	counts := q.TargetCounts()
	if len(counts) != 0 {
		t.Errorf("TargetCounts() = %v, want empty", counts)
	}
}

func TestQueue_PushAndDrain(t *testing.T) {
	q := NewQueue("test")
	q.Push(MessageJob{Target: "user1", Type: "discord:user"})
	q.Push(MessageJob{Target: "user2", Type: "discord:user"})
	q.Push(MessageJob{Target: "user1", Type: "discord:user"})

	if q.Len() != 3 {
		t.Errorf("Len() = %d, want 3", q.Len())
	}

	jobs := q.Drain()
	if len(jobs) != 3 {
		t.Fatalf("Drain() returned %d jobs, want 3", len(jobs))
	}
	if q.Len() != 0 {
		t.Errorf("Len() after Drain = %d, want 0", q.Len())
	}
}

func TestQueue_TargetCounts(t *testing.T) {
	q := NewQueue("test")
	q.Push(MessageJob{Target: "user1", Type: "discord:user"})
	q.Push(MessageJob{Target: "user1", Type: "discord:user"})
	q.Push(MessageJob{Target: "user2", Type: "discord:user"})
	q.Push(MessageJob{Target: "", Type: "discord:user"}) // blank target should be skipped

	counts := q.TargetCounts()
	if counts["user1"] != 2 {
		t.Errorf("counts[user1] = %d, want 2", counts["user1"])
	}
	if counts["user2"] != 1 {
		t.Errorf("counts[user2] = %d, want 1", counts["user2"])
	}
	if _, ok := counts[""]; ok {
		t.Error("blank target should not appear in counts")
	}
}
