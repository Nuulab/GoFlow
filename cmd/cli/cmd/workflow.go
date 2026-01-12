package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(workflowCmd)

	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowStartCmd)
	workflowCmd.AddCommand(workflowStatusCmd)
	workflowCmd.AddCommand(workflowSignalCmd)
	workflowCmd.AddCommand(workflowPauseCmd)
	workflowCmd.AddCommand(workflowResumeCmd)

	workflowStartCmd.Flags().StringP("input", "i", "{}", "Initial input JSON")
	workflowSignalCmd.Flags().StringP("data", "d", "{}", "Signal data JSON")
}

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Manage workflows",
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active workflows",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(bold("🔄 Active Workflows"))
		fmt.Println()

		client := NewAPIClient()
		var workflows []struct {
			ID        string    `json:"id"`
			Status    string    `json:"status"`
			Name      string    `json:"name,omitempty"`
			CreatedAt time.Time `json:"created_at"`
		}

		// Assuming API endpoint exists
		if err := client.Get("/api/workflows", &workflows); err != nil {
			fail(fmt.Sprintf("Failed to list workflows: %v", err))
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tSTARTED")
		fmt.Fprintln(w, "--\t----\t------\t-------")

		for _, wf := range workflows {
			statusColor := cyan
			if wf.Status == "failed" {
				statusColor = red
			} else if wf.Status == "completed" {
				statusColor = green
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				wf.ID,
				wf.Name,
				statusColor(wf.Status),
				wf.CreatedAt.Format("15:04:05"),
			)
		}
		w.Flush()
	},
}

var workflowStartCmd = &cobra.Command{
	Use:   "start <workflow-id>",
	Short: "Start a new workflow instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		inputStr, _ := cmd.Flags().GetString("input")

		var input interface{}
		if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
			fail("Invalid input JSON")
			return
		}

		client := NewAPIClient()
		if err := client.Post(fmt.Sprintf("/api/workflows/%s/start", id), input); err != nil {
			fail(fmt.Sprintf("Failed to start workflow: %v", err))
			return
		}

		// If the response returns an ID (it might if we generate one)
		// Or we use the provided ID if it's deterministic
		success(fmt.Sprintf("Workflow started: %s", cyan(id)))
	},
}

var workflowStatusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Get workflow status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := NewAPIClient()
		var wf struct {
			ID        string                 `json:"id"`
			Status    string                 `json:"status"`
			Result    interface{}            `json:"result,omitempty"`
			History   []string               `json:"history"` // Simplified history
			Variables map[string]interface{} `json:"variables"`
		}

		if err := client.Get(fmt.Sprintf("/api/workflows/%s", id), &wf); err != nil {
			fail(fmt.Sprintf("Failed to get status: %v", err))
			return
		}

		fmt.Printf(bold("Workflow: %s\n"), cyan(wf.ID))
		fmt.Printf("Status: %s\n", wf.Status)
		if wf.Result != nil {
			res, _ := json.MarshalIndent(wf.Result, "", "  ")
			fmt.Printf("Result: %s\n", string(res))
		}
	},
}

var workflowSignalCmd = &cobra.Command{
	Use:   "signal <id> <signal-name>",
	Short: "Send a signal to a workflow",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		signalName := args[1]
		dataStr, _ := cmd.Flags().GetString("data")

		var data interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			fail("Invalid data JSON")
			return
		}

		client := NewAPIClient()
		payload := map[string]interface{}{
			"signal": signalName,
			"data":   data,
		}

		if err := client.Post(fmt.Sprintf("/api/workflows/%s/signal", id), payload); err != nil {
			fail(fmt.Sprintf("Failed to send signal: %v", err))
			return
		}

		success(fmt.Sprintf("Signal %s sent to %s", cyan(signalName), cyan(id)))
	},
}

var workflowPauseCmd = &cobra.Command{
	Use:   "pause <id>",
	Short: "Pause a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := NewAPIClient()
		if err := client.Post(fmt.Sprintf("/api/workflows/%s/pause", id), nil); err != nil {
			fail(fmt.Sprintf("Failed to pause: %v", err))
			return
		}
		success(fmt.Sprintf("Workflow %s paused", cyan(id)))
	},
}

var workflowResumeCmd = &cobra.Command{
	Use:   "resume <id>",
	Short: "Resume a paused workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		client := NewAPIClient()
		if err := client.Post(fmt.Sprintf("/api/workflows/%s/resume", id), nil); err != nil {
			fail(fmt.Sprintf("Failed to resume: %v", err))
			return
		}
		success(fmt.Sprintf("Workflow %s resumed", cyan(id)))
	},
}
