// Package workflow provides cron scheduling for workflows.
package workflow

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Cron manages scheduled workflow executions.
type Cron struct {
	engine    *Engine
	schedules map[string]*Schedule
	stop      chan struct{}
	running   bool
	mu        sync.RWMutex
}

// Schedule represents a cron schedule.
type Schedule struct {
	ID           string
	WorkflowName string
	Expression   string
	Input        map[string]any
	Enabled      bool
	LastRun      time.Time
	NextRun      time.Time
	parsed       *CronExpression
}

// NewCron creates a new cron scheduler.
func NewCron(engine *Engine) *Cron {
	return &Cron{
		engine:    engine,
		schedules: make(map[string]*Schedule),
		stop:      make(chan struct{}),
	}
}

// Add adds a scheduled workflow.
func (c *Cron) Add(id, workflowName, expression string, input map[string]any) error {
	parsed, err := ParseCron(expression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	schedule := &Schedule{
		ID:           id,
		WorkflowName: workflowName,
		Expression:   expression,
		Input:        input,
		Enabled:      true,
		parsed:       parsed,
		NextRun:      parsed.Next(time.Now()),
	}

	c.schedules[id] = schedule
	return nil
}

// Remove removes a scheduled workflow.
func (c *Cron) Remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.schedules, id)
}

// Enable enables a schedule.
func (c *Cron) Enable(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.schedules[id]; ok {
		s.Enabled = true
		s.NextRun = s.parsed.Next(time.Now())
	}
}

// Disable disables a schedule.
func (c *Cron) Disable(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.schedules[id]; ok {
		s.Enabled = false
	}
}

// List returns all schedules.
func (c *Cron) List() []*Schedule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(c.schedules))
	for _, s := range c.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// Start starts the cron scheduler.
func (c *Cron) Start(ctx context.Context) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stop = make(chan struct{})
	c.mu.Unlock()

	go c.run(ctx)
}

// Stop stops the cron scheduler.
func (c *Cron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		close(c.stop)
		c.running = false
	}
}

func (c *Cron) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		case now := <-ticker.C:
			c.checkSchedules(ctx, now)
		}
	}
}

func (c *Cron) checkSchedules(ctx context.Context, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, schedule := range c.schedules {
		if !schedule.Enabled {
			continue
		}

		if now.After(schedule.NextRun) || now.Equal(schedule.NextRun) {
			// Trigger workflow
			go c.triggerWorkflow(ctx, schedule)

			// Update schedule
			schedule.LastRun = now
			schedule.NextRun = schedule.parsed.Next(now)
		}
	}
}

func (c *Cron) triggerWorkflow(ctx context.Context, schedule *Schedule) {
	input := make(map[string]any)
	for k, v := range schedule.Input {
		input[k] = v
	}
	input["_cron_schedule_id"] = schedule.ID
	input["_cron_triggered_at"] = time.Now()

	_, err := c.engine.Start(ctx, schedule.WorkflowName, input)
	if err != nil {
		// Log error (in production, use proper logging)
		fmt.Printf("Cron: failed to start workflow %s: %v\n", schedule.WorkflowName, err)
	}
}

// ============ Cron Expression Parser ============

// CronExpression represents a parsed cron expression.
type CronExpression struct {
	minute     []int // 0-59
	hour       []int // 0-23
	dayOfMonth []int // 1-31
	month      []int // 1-12
	dayOfWeek  []int // 0-6 (Sunday = 0)
}

// ParseCron parses a cron expression.
// Supports: * */n n n-m n,m
// Format: minute hour day-of-month month day-of-week
func ParseCron(expression string) (*CronExpression, error) {
	// Handle special expressions
	switch expression {
	case "@yearly", "@annually":
		expression = "0 0 1 1 *"
	case "@monthly":
		expression = "0 0 1 * *"
	case "@weekly":
		expression = "0 0 * * 0"
	case "@daily", "@midnight":
		expression = "0 0 * * *"
	case "@hourly":
		expression = "0 * * * *"
	}

	// Handle @every expressions
	if strings.HasPrefix(expression, "@every ") {
		duration := strings.TrimPrefix(expression, "@every ")
		return parseEvery(duration)
	}

	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	expr := &CronExpression{}
	var err error

	expr.minute, err = parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}

	expr.hour, err = parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}

	expr.dayOfMonth, err = parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day of month: %w", err)
	}

	expr.month, err = parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}

	expr.dayOfWeek, err = parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day of week: %w", err)
	}

	return expr, nil
}

func parseEvery(duration string) (*CronExpression, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}

	minutes := int(d.Minutes())
	if minutes <= 0 {
		return nil, fmt.Errorf("duration must be at least 1 minute")
	}

	if minutes < 60 {
		// Every N minutes
		mins := make([]int, 0)
		for i := 0; i < 60; i += minutes {
			mins = append(mins, i)
		}
		return &CronExpression{
			minute:     mins,
			hour:       makeRange(0, 23),
			dayOfMonth: makeRange(1, 31),
			month:      makeRange(1, 12),
			dayOfWeek:  makeRange(0, 6),
		}, nil
	}

	hours := minutes / 60
	if hours < 24 {
		// Every N hours
		hrs := make([]int, 0)
		for i := 0; i < 24; i += hours {
			hrs = append(hrs, i)
		}
		return &CronExpression{
			minute:     []int{0},
			hour:       hrs,
			dayOfMonth: makeRange(1, 31),
			month:      makeRange(1, 12),
			dayOfWeek:  makeRange(0, 6),
		}, nil
	}

	return nil, fmt.Errorf("duration too long, use standard cron expression")
}

func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		return makeRange(min, max), nil
	}

	// Handle */n (every n)
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		if err != nil {
			return nil, err
		}
		values := make([]int, 0)
		for i := min; i <= max; i += step {
			values = append(values, i)
		}
		return values, nil
	}

	// Handle ranges and lists
	var values []int
	for _, part := range strings.Split(field, ",") {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, err
			}
			for i := start; i <= end; i++ {
				values = append(values, i)
			}
		} else {
			val, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
	}

	// Validate
	for _, v := range values {
		if v < min || v > max {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", v, min, max)
		}
	}

	sort.Ints(values)
	return values, nil
}

func makeRange(min, max int) []int {
	values := make([]int, max-min+1)
	for i := range values {
		values[i] = min + i
	}
	return values
}

// Next returns the next time that matches the cron expression.
func (c *CronExpression) Next(from time.Time) time.Time {
	// Start from the next minute
	t := from.Add(time.Minute).Truncate(time.Minute)

	for i := 0; i < 366*24*60; i++ { // Search up to 1 year
		if c.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}

	return time.Time{} // No match found
}

func (c *CronExpression) matches(t time.Time) bool {
	if !contains(c.minute, t.Minute()) {
		return false
	}
	if !contains(c.hour, t.Hour()) {
		return false
	}
	if !contains(c.dayOfMonth, t.Day()) {
		return false
	}
	if !contains(c.month, int(t.Month())) {
		return false
	}
	if !contains(c.dayOfWeek, int(t.Weekday())) {
		return false
	}
	return true
}

func contains(values []int, v int) bool {
	for _, val := range values {
		if val == v {
			return true
		}
	}
	return false
}

// ============ Schedule Helpers ============

// Every creates a schedule that runs every duration.
func Every(duration time.Duration) string {
	return fmt.Sprintf("@every %s", duration)
}

// At creates a schedule for a specific time (daily).
func At(hour, minute int) string {
	return fmt.Sprintf("%d %d * * *", minute, hour)
}

// Daily creates a schedule that runs daily at midnight.
func Daily() string {
	return "@daily"
}

// Hourly creates a schedule that runs every hour.
func Hourly() string {
	return "@hourly"
}

// Weekly creates a schedule that runs weekly on Sunday.
func Weekly() string {
	return "@weekly"
}

// Monthly creates a schedule that runs monthly on the 1st.
func Monthly() string {
	return "@monthly"
}

// Weekdays creates a schedule that runs on weekdays at the given time.
func Weekdays(hour, minute int) string {
	return fmt.Sprintf("%d %d * * 1-5", minute, hour)
}

// Weekends creates a schedule that runs on weekends at the given time.
func Weekends(hour, minute int) string {
	return fmt.Sprintf("%d %d * * 0,6", minute, hour)
}
