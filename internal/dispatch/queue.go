package dispatch

import "sync"

// MessageJob captures a message dispatch request.
type MessageJob struct {
	Lat          float64
	Lon          float64
	Message      string
	Payload      map[string]any
	Target       string
	Type         string
	Name         string
	TTH          TimeToHide
	Clean        bool
	Emoji        string
	LogReference string
	Language     string
	AlwaysSend   bool
	UpdateKey    string
	UpdateExisting bool
}

// TimeToHide mirrors PoracleJS tth object.
type TimeToHide struct {
	Hours   int
	Minutes int
	Seconds int
}

// Queue keeps message jobs in memory until workers consume them.
type Queue struct {
	name string
	mu   sync.Mutex
	jobs []MessageJob
}

// NewQueue constructs a new in-memory queue.
func NewQueue(name string) *Queue {
	return &Queue{name: name, jobs: []MessageJob{}}
}

// Push adds a job to the queue.
func (q *Queue) Push(job MessageJob) {
	q.mu.Lock()
	q.jobs = append(q.jobs, job)
	q.mu.Unlock()
}

// Drain returns and clears queued jobs.
func (q *Queue) Drain() []MessageJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]MessageJob, len(q.jobs))
	copy(out, q.jobs)
	q.jobs = q.jobs[:0]
	return out
}

// Len returns the number of queued jobs.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// TargetCounts returns queued job counts by target.
func (q *Queue) TargetCounts() map[string]int {
	q.mu.Lock()
	defer q.mu.Unlock()
	counts := map[string]int{}
	for _, job := range q.jobs {
		if job.Target == "" {
			continue
		}
		counts[job.Target]++
	}
	return counts
}
