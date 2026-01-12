# Webhooks

Webhooks allow external services to trigger jobs and workflows in GoFlow.

## Overview

Webhooks are HTTP endpoints that receive POST requests from external services (GitHub, Stripe, Slack, etc.) and automatically:
- Enqueue jobs for background processing
- Start workflows
- Send signals to running workflows

## Basic Setup

```go
import (
    "net/http"
    "github.com/goflow/goflow/pkg/webhook"
    "github.com/goflow/goflow/pkg/queue"
    "github.com/goflow/goflow/pkg/workflow"
)

// Create webhook handler with queue and workflow engine
handler := webhook.NewWebhookHandler(myQueue, myEngine)

// Mount at /webhooks
http.Handle("/webhooks/", handler.Handler())
```

The handler listens for POST requests and routes them to the appropriate action based on the path.

## Job Webhooks

Trigger background jobs from webhooks:

```go
// Register webhook → job mapping
handler.RegisterJobWebhook("/github", "process_github_event")
handler.RegisterJobWebhook("/stripe", "handle_stripe_event")
handler.RegisterJobWebhook("/slack", "process_slack_command")
```

When a POST arrives at `/webhooks/github`, GoFlow:
1. Parses the JSON payload
2. Creates a job of type `process_github_event`
3. Adds the payload as job data
4. Enqueues it for processing

Your worker then handles it:

```go
worker.Handle("process_github_event", func(ctx context.Context, job *queue.Job) error {
    var payload map[string]any
    job.UnmarshalPayload(&payload)
    
    event := payload["event"].(string)
    // Process the GitHub event...
    return nil
})
```

## Workflow Webhooks

Start workflows from external events:

```go
handler.RegisterWorkflowWebhook("/order", "order-process")
handler.RegisterWorkflowWebhook("/signup", "user-onboarding")
```

When Stripe sends an order webhook to `/webhooks/order`, GoFlow automatically starts the `order-process` workflow with the payload data as input.

## Signature Validation

Validate webhook signatures for security:

```go
// Global secret for all webhooks
handler.SetGlobalSecret("whsec_abc123")

// Or per-webhook secret
handler.Register(&webhook.WebhookConfig{
    Path:    "/stripe",
    Secret:  os.Getenv("STRIPE_WEBHOOK_SECRET"),
    Action:  webhook.ActionEnqueueJob,
    JobType: "stripe_event",
})
```

GoFlow validates the `X-Webhook-Signature` header using HMAC-SHA256. Invalid signatures are rejected with 401 Unauthorized.

## Custom Payload Transform

Transform incoming webhooks before processing:

```go
handler.Register(&webhook.WebhookConfig{
    Path:    "/custom",
    Action:  webhook.ActionEnqueueJob,
    JobType: "custom_event",
    Transform: func(body []byte) any {
        // Parse and transform the raw webhook body
        var raw map[string]any
        json.Unmarshal(body, &raw)
        
        return map[string]any{
            "user_id": raw["user"]["id"],
            "action":  raw["event_type"],
        }
    },
})
```

The `Transform` function lets you reshape the incoming payload before it becomes job/workflow input.

## Sending Webhooks

Agents can send outgoing webhooks:

```go
import "github.com/goflow/goflow/pkg/webhook"

tool := webhook.NewWebhookTool(handler)

result, err := tool.SendWebhook(ctx, webhook.SendWebhookInput{
    URL:   "https://api.example.com/hooks/notify",
    Event: "order.shipped",
    Data: map[string]any{
        "order_id": "12345",
        "tracking": "TRACK123",
    },
})
```

This sends a POST request with the payload and returns the response.

## Managing Webhooks

```go
// List all webhooks
hooks := handler.List()

// Disable a webhook (keeps config but rejects requests)
handler.Disable("/github")

// Re-enable
handler.Enable("/github")

// Remove completely
handler.Remove("/github")
```

## API Endpoints

The API server exposes webhook management:

```
GET    /api/webhooks        List registered webhooks
POST   /api/webhooks        Register new webhook
DELETE /api/webhooks/:path  Remove webhook
PUT    /api/webhooks/:path  Enable/disable webhook
```

## Example: GitHub → Workflow

Complete example of GitHub webhooks triggering deployments:

```go
// Register webhook
handler.Register(&webhook.WebhookConfig{
    Path:       "/github-deploy",
    Secret:     os.Getenv("GITHUB_WEBHOOK_SECRET"),
    Action:     webhook.ActionStartWorkflow,
    WorkflowID: "deploy-workflow",
    Transform: func(body []byte) any {
        var gh map[string]any
        json.Unmarshal(body, &gh)
        
        return map[string]any{
            "repo":   gh["repository"].(map[string]any)["full_name"],
            "branch": strings.TrimPrefix(gh["ref"].(string), "refs/heads/"),
            "commit": gh["after"],
        }
    },
})
```

When GitHub sends a push event:
1. GoFlow validates the signature
2. Transforms the payload to extract repo, branch, commit
3. Starts the `deploy-workflow` with that input
4. Returns success to GitHub

The workflow then handles the actual deployment.
