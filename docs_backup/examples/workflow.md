# Workflow Example

Build a complete order processing workflow.

## Code

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/goflow/goflow/pkg/workflow"
)

func main() {
	// Define the workflow
	orderWorkflow := workflow.New("order-process").
		// Step 1: Validate the order
		Step("validate", validateOrder).Then().
		
		// Step 2: Check inventory (with retry)
		Step("check_inventory", checkInventory).
			Retry(workflow.NewRetryPolicy().Attempts(3)).
			Then().
		
		// Step 3: Conditional - high value orders need approval
		If("needs_approval", func(s *workflow.State) bool {
			return s.Data["total"].(float64) > 1000
		}).
			Then(workflow.Step("get_approval", getApproval).
				AwaitApproval("manager", []string{"manager@company.com"}).
				Timeout(24 * time.Hour)).
			Else(workflow.Step("auto_approve", autoApprove)).
		End().
		
		// Step 4: Process payment (with compensation)
		Step("charge_card", chargeCard).
			Compensate(refundCard).
			Then().
		
		// Step 5: Parallel - fulfill and notify
		Parallel("fulfill_and_notify",
			workflow.Step("ship_order", shipOrder),
			workflow.Step("send_confirmation", sendConfirmation),
		).WaitFor(workflow.WaitAll).
		
		Build()

	// Create engine and register workflow
	engine := workflow.NewEngine(nil)
	engine.Register(orderWorkflow)

	// Start the workflow
	stateID, _ := engine.Start(context.Background(), "order-process", map[string]any{
		"order_id":    12345,
		"customer_id": "cust-789",
		"items":       []string{"widget-a", "gadget-b"},
		"total":       1500.00,
	})

	fmt.Println("Started workflow:", stateID)

	// Monitor progress
	for {
		status, _ := engine.Status(stateID)
		fmt.Printf("Status: %s, State: %s\n", status.Status, status.CurrentState)
		
		if status.Status == workflow.StatusCompleted || 
		   status.Status == workflow.StatusFailed {
			break
		}
		time.Sleep(time.Second)
	}
}

// Step functions
func validateOrder(ctx context.Context, s *workflow.State) error {
	fmt.Println("Validating order", s.Data["order_id"])
	// Validation logic
	return nil
}

func checkInventory(ctx context.Context, s *workflow.State) error {
	fmt.Println("Checking inventory")
	// Check stock levels
	s.Data["in_stock"] = true
	return nil
}

func getApproval(ctx context.Context, s *workflow.State) error {
	fmt.Println("Waiting for manager approval...")
	// This pauses until approval is granted
	return nil
}

func autoApprove(ctx context.Context, s *workflow.State) error {
	fmt.Println("Auto-approved (under $1000)")
	return nil
}

func chargeCard(ctx context.Context, s *workflow.State) error {
	fmt.Println("Charging card for $", s.Data["total"])
	s.Data["transaction_id"] = "txn-" + time.Now().Format("20060102150405")
	return nil
}

func refundCard(ctx context.Context, s *workflow.State) error {
	fmt.Println("Refunding transaction", s.Data["transaction_id"])
	return nil
}

func shipOrder(ctx context.Context, s *workflow.State) error {
	fmt.Println("Shipping order to customer")
	s.Data["tracking_number"] = "TRACK123456"
	return nil
}

func sendConfirmation(ctx context.Context, s *workflow.State) error {
	fmt.Println("Sending confirmation email")
	return nil
}
```

## Workflow Features Used

1. **Sequential Steps** - Steps run in order
2. **Retry Policies** - Auto-retry on failure
3. **Conditionals** - Branch based on data
4. **Human Approval** - Wait for external approval
5. **Compensation** - Rollback on failure
6. **Parallel Execution** - Run steps concurrently

## External Approval

Approve from outside the workflow:

```go
// Manager approves via API/CLI
engine.Approve(stateID, "manager")

// Or reject
engine.Reject(stateID, "manager")
```
