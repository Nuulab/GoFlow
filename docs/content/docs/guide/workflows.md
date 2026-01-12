---
title: Workflows
---


Workflows define complex multi-step processes with conditionals, loops, and human approvals.

## Basic Workflow

```go
import "github.com/nuulab/goflow/pkg/workflow"

wf := workflow.New("order-process").
    Step("validate", validateOrder).Then().
    Step("charge", chargeCard).Then().
    Step("fulfill", fulfillOrder).Then().
    Step("notify", sendNotification).
    Build()
```

## Running Workflows

```go
engine := workflow.NewEngine(nil)
engine.Register(wf)

// Start workflow
stateID, err := engine.Start(ctx, "order-process", map[string]any{
    "order_id": 12345,
})

// Check status
state, _ := engine.Status(stateID)
fmt.Println(state.Status) // running, completed, failed
```

## Conditionals

```go
workflow.New("approval").
    Step("validate", validateRequest).Then().
    If("needs_approval", func(s *State) bool {
        return s.Data["amount"].(float64) > 1000
    }).
        Then(workflow.Step("approve", getApproval)).
        Else(workflow.Step("auto_approve", autoApprove)).
    End().
    Step("process", processRequest).
    Build()
```

## Loops

### ForEach Loop

```go
workflow.New("batch-process").
    Loop("process_items").
        ForEach("items").
        Do(workflow.Step("process", processItem)).
    End().
    Build()
```

### While Loop

```go
workflow.New("retry-until").
    Loop("retry").
        While(func(s *State) bool {
            return s.Data["retries"].(int) < 3
        }).
        Do(workflow.Step("attempt", attemptOperation)).
    End().
    Build()
```

## Parallel Execution

```go
workflow.New("fetch-all").
    Parallel("gather",
        workflow.Step("users", fetchUsers),
        workflow.Step("orders", fetchOrders),
        workflow.Step("inventory", fetchInventory),
    ).WaitFor(workflow.WaitAll).
    Step("combine", combineResults).
    Build()
```

Wait strategies:
- `WaitAll` - Wait for all parallel steps
- `WaitAny` - Continue when any step completes
- `WaitCount(n)` - Wait for n steps to complete

## Human-in-the-Loop

```go
workflow.New("expense-approval").
    Step("submit", submitExpense).Then().
    AwaitApproval("manager_approval", []string{"manager@company.com"}).
        Timeout(24 * time.Hour).
        OnTimeout("auto_reject").
    Then().
    Step("process", processExpense).
    Build()
```

## Compensation (Saga Pattern)

```go
workflow.New("booking").
    Step("charge", chargeCard).
        Compensate(refundCard).
    Then().
    Step("reserve", reserveRoom).
        Compensate(cancelReservation).
    Then().
    Step("confirm", sendConfirmation).
    Build()

// If reserveRoom fails, refundCard is automatically called
```

## Signals & Events

```go
// Wait for external signal
workflow.New("async-process").
    Step("start", startProcess).Then().
    AwaitSignal("payment_received").
        Timeout(1 * time.Hour).
    Then().
    Step("complete", completeProcess).
    Build()

// Send signal from outside
engine.Signal(stateID, "payment_received", paymentData)
```

## Sub-Workflows

```go
workflow.New("main").
    Step("init", initialize).Then().
    SubWorkflow("process", "sub-workflow-name", subInput).Then().
    Step("finalize", finalize).
    Build()
```

## Retry Policies

```go
workflow.New("resilient").
    Step("external_call", callAPI).
        Retry(workflow.NewRetryPolicy().
            Attempts(5).
            Backoff(time.Second, time.Minute).
            RetryOn(isRetryableError),
        ).
    Build()
```

## Cron Scheduling

```go
cron := workflow.NewCron(engine)

// Standard cron
cron.Add("daily-report", "report_workflow", "0 9 * * *", nil)

// Special expressions
cron.Add("hourly-sync", "sync_workflow", "@hourly", nil)
cron.Add("every-5m", "health_check", "@every 5m", nil)

cron.Start(ctx)
```
