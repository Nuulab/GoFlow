// Package metrics provides Prometheus instrumentation for GoFlow.
package metrics

import (
	"net/http"
	"time"
)

// Note: This is a minimal implementation without prometheus dependency.
// To use real Prometheus, add: github.com/prometheus/client_golang

// Metrics holds all GoFlow metrics.
type Metrics struct {
	// Jobs
	JobsEnqueued    *Counter
	JobsDequeued    *Counter
	JobsCompleted   *Counter
	JobsFailed      *Counter
	JobsRetried     *Counter
	JobsDLQ         *Counter
	JobDuration     *Histogram
	QueueDepth      *Gauge
	
	// Workers
	WorkersActive   *Gauge
	WorkersBusy     *Gauge
	
	// Agents
	AgentRuns       *Counter
	AgentSteps      *Counter
	AgentToolCalls  *Counter
	
	// Workflows
	WorkflowsStarted   *Counter
	WorkflowsCompleted *Counter
	WorkflowsFailed    *Counter
	WorkflowDuration   *Histogram
	
	// System
	Uptime          *Gauge
	MemoryUsage     *Gauge
	GoroutineCount  *Gauge
}

// Counter is a monotonically increasing counter.
type Counter struct {
	name   string
	labels map[string]string
	value  float64
}

// Gauge is a value that can go up or down.
type Gauge struct {
	name   string
	labels map[string]string
	value  float64
}

// Histogram tracks distribution of values.
type Histogram struct {
	name    string
	labels  map[string]string
	count   uint64
	sum     float64
	buckets []float64
}

// NewMetrics creates a new metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		// Jobs
		JobsEnqueued:    NewCounter("goflow_jobs_enqueued_total", "Total jobs enqueued"),
		JobsDequeued:    NewCounter("goflow_jobs_dequeued_total", "Total jobs dequeued"),
		JobsCompleted:   NewCounter("goflow_jobs_completed_total", "Total jobs completed"),
		JobsFailed:      NewCounter("goflow_jobs_failed_total", "Total jobs failed"),
		JobsRetried:     NewCounter("goflow_jobs_retried_total", "Total jobs retried"),
		JobsDLQ:         NewCounter("goflow_jobs_dlq_total", "Total jobs sent to DLQ"),
		JobDuration:     NewHistogram("goflow_job_duration_seconds", "Job processing duration"),
		QueueDepth:      NewGauge("goflow_queue_depth", "Current queue depth"),
		
		// Workers
		WorkersActive:   NewGauge("goflow_workers_active", "Number of active workers"),
		WorkersBusy:     NewGauge("goflow_workers_busy", "Number of busy workers"),
		
		// Agents
		AgentRuns:       NewCounter("goflow_agent_runs_total", "Total agent runs"),
		AgentSteps:      NewCounter("goflow_agent_steps_total", "Total agent steps"),
		AgentToolCalls:  NewCounter("goflow_agent_tool_calls_total", "Total tool calls"),
		
		// Workflows
		WorkflowsStarted:   NewCounter("goflow_workflows_started_total", "Total workflows started"),
		WorkflowsCompleted: NewCounter("goflow_workflows_completed_total", "Total workflows completed"),
		WorkflowsFailed:    NewCounter("goflow_workflows_failed_total", "Total workflows failed"),
		WorkflowDuration:   NewHistogram("goflow_workflow_duration_seconds", "Workflow duration"),
		
		// System
		Uptime:         NewGauge("goflow_uptime_seconds", "Process uptime"),
		MemoryUsage:    NewGauge("goflow_memory_bytes", "Memory usage"),
		GoroutineCount: NewGauge("goflow_goroutines", "Number of goroutines"),
	}
}

// NewCounter creates a new counter.
func NewCounter(name, help string) *Counter {
	return &Counter{name: name, labels: make(map[string]string)}
}

// NewGauge creates a new gauge.
func NewGauge(name, help string) *Gauge {
	return &Gauge{name: name, labels: make(map[string]string)}
}

// NewHistogram creates a new histogram.
func NewHistogram(name, help string) *Histogram {
	return &Histogram{
		name:    name,
		labels:  make(map[string]string),
		buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}
}

// Inc increments a counter by 1.
func (c *Counter) Inc() {
	c.value++
}

// Add adds a value to a counter.
func (c *Counter) Add(v float64) {
	c.value += v
}

// WithLabels returns a counter with labels.
func (c *Counter) WithLabels(labels map[string]string) *Counter {
	return &Counter{name: c.name, labels: labels, value: 0}
}

// Value returns the current value.
func (c *Counter) Value() float64 {
	return c.value
}

// Set sets a gauge value.
func (g *Gauge) Set(v float64) {
	g.value = v
}

// Inc increments a gauge by 1.
func (g *Gauge) Inc() {
	g.value++
}

// Dec decrements a gauge by 1.
func (g *Gauge) Dec() {
	g.value--
}

// Add adds to a gauge.
func (g *Gauge) Add(v float64) {
	g.value += v
}

// Value returns the current value.
func (g *Gauge) Value() float64 {
	return g.value
}

// Observe records a value in the histogram.
func (h *Histogram) Observe(v float64) {
	h.count++
	h.sum += v
}

// ObserveDuration records a duration.
func (h *Histogram) ObserveDuration(start time.Time) {
	h.Observe(time.Since(start).Seconds())
}

// Count returns the number of observations.
func (h *Histogram) Count() uint64 {
	return h.count
}

// Sum returns the sum of observations.
func (h *Histogram) Sum() float64 {
	return h.sum
}

// Avg returns the average.
func (h *Histogram) Avg() float64 {
	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

// Handler returns an HTTP handler for metrics.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		
		// Jobs
		writeMetric(w, "goflow_jobs_enqueued_total", m.JobsEnqueued.Value())
		writeMetric(w, "goflow_jobs_dequeued_total", m.JobsDequeued.Value())
		writeMetric(w, "goflow_jobs_completed_total", m.JobsCompleted.Value())
		writeMetric(w, "goflow_jobs_failed_total", m.JobsFailed.Value())
		writeMetric(w, "goflow_jobs_retried_total", m.JobsRetried.Value())
		writeMetric(w, "goflow_jobs_dlq_total", m.JobsDLQ.Value())
		writeMetric(w, "goflow_queue_depth", m.QueueDepth.Value())
		
		// Job duration histogram
		writeMetric(w, "goflow_job_duration_seconds_count", float64(m.JobDuration.Count()))
		writeMetric(w, "goflow_job_duration_seconds_sum", m.JobDuration.Sum())
		
		// Workers
		writeMetric(w, "goflow_workers_active", m.WorkersActive.Value())
		writeMetric(w, "goflow_workers_busy", m.WorkersBusy.Value())
		
		// Agents
		writeMetric(w, "goflow_agent_runs_total", m.AgentRuns.Value())
		writeMetric(w, "goflow_agent_steps_total", m.AgentSteps.Value())
		writeMetric(w, "goflow_agent_tool_calls_total", m.AgentToolCalls.Value())
		
		// Workflows
		writeMetric(w, "goflow_workflows_started_total", m.WorkflowsStarted.Value())
		writeMetric(w, "goflow_workflows_completed_total", m.WorkflowsCompleted.Value())
		writeMetric(w, "goflow_workflows_failed_total", m.WorkflowsFailed.Value())
		
		// System
		writeMetric(w, "goflow_uptime_seconds", m.Uptime.Value())
		writeMetric(w, "goflow_memory_bytes", m.MemoryUsage.Value())
		writeMetric(w, "goflow_goroutines", m.GoroutineCount.Value())
	})
}

func writeMetric(w http.ResponseWriter, name string, value float64) {
	w.Write([]byte(name + " " + formatFloat(value) + "\n"))
}

func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return string(rune(int64(v) + '0'))
	}
	return string(rune(int64(v)))
}

// Global metrics instance
var DefaultMetrics = NewMetrics()

// Convenience functions using default metrics
func JobEnqueued()  { DefaultMetrics.JobsEnqueued.Inc() }
func JobDequeued()  { DefaultMetrics.JobsDequeued.Inc() }
func JobCompleted() { DefaultMetrics.JobsCompleted.Inc() }
func JobFailed()    { DefaultMetrics.JobsFailed.Inc() }
func JobRetried()   { DefaultMetrics.JobsRetried.Inc() }
func JobToDLQ()     { DefaultMetrics.JobsDLQ.Inc() }

func ObserveJobDuration(start time.Time) {
	DefaultMetrics.JobDuration.ObserveDuration(start)
}

func SetQueueDepth(v float64) {
	DefaultMetrics.QueueDepth.Set(v)
}
