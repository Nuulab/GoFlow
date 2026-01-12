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
	rootCmd.AddCommand(jobCmd)

	// Sub-commands
	jobCmd.AddCommand(jobListCmd)
	jobCmd.AddCommand(jobEnqueueCmd)
	jobCmd.AddCommand(jobRetryCmd)
	jobCmd.AddCommand(jobStatusCmd)

	// Enqueue flags
	jobEnqueueCmd.Flags().StringP("type", "t", "", "job type (required)")
	jobEnqueueCmd.Flags().StringP("payload", "d", "{}", "job payload (JSON)")
	jobEnqueueCmd.Flags().IntP("priority", "p", 0, "job priority")
	jobEnqueueCmd.MarkFlagRequired("type")

	// List flags
	jobListCmd.Flags().StringP("status", "s", "", "filter by status")
	jobListCmd.Flags().IntP("limit", "n", 20, "max jobs to show")
}

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs",
	Long:  `Create, list, and manage queued jobs.`,
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs in queue",
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		status, _ := cmd.Flags().GetString("status")

		fmt.Println(bold("📋 Jobs in Queue"))
		if status != "" {
			fmt.Printf("   Filtering by status: %s\n", cyan(status))
		}
		fmt.Println()

		client := NewAPIClient()
		var jobs []struct {
			IDString  string    `json:"id"`
			Type      string    `json:"name"`
			Status    string    `json:"status"`
			Priority  int       `json:"priority"`
			CreatedAt time.Time `json:"created_at"`
		}

		query := fmt.Sprintf("/api/queue/jobs?limit=%d", limit)
		if status != "" {
			query += fmt.Sprintf("&status=%s", status)
		}

		if err := client.Get(query, &jobs); err != nil {
			fail(fmt.Sprintf("Failed to list jobs: %v", err))
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tPRIORITY\tCREATED")
		fmt.Fprintln(w, "---\t----\t------\t--------\t-------")

		for _, j := range jobs {
			statusColor := green
			if j.Status == "failed" {
				statusColor = red
			} else if j.Status == "pending" {
				statusColor = yellow
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				cyan(j.IDString),
				j.Type,
				statusColor(j.Status),
				j.Priority,
				j.CreatedAt.Format("15:04:05"),
			)
		}
		w.Flush()
	},
}

var jobEnqueueCmd = &cobra.Command{
	Use:   "enqueue",
	Short: "Add a job to the queue",
	Run: func(cmd *cobra.Command, args []string) {
		jobType, _ := cmd.Flags().GetString("type")
		payloadStr, _ := cmd.Flags().GetString("payload")
		priority, _ := cmd.Flags().GetInt("priority")

		// Validate JSON
		var payload interface{}
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			fail(fmt.Sprintf("Invalid JSON payload: %v", err))
			return
		}

		client := NewAPIClient()
		reqBody := map[string]interface{}{
			"name":     jobType,
			"data":     payload,
			"priority": priority,
		}

		if err := client.Post("/api/queue/enqueue", reqBody); err != nil {
			// If Post doesn't decode response, we might miss the ID. 
			// But our simplified Post helper doesn't decode.
			// Ideally update Post to accept a response target, but for now just success
			// If needed we can improve the client later
			fail(fmt.Sprintf("Failed to enqueue: %v", err))
			return
		}

		// Since our simple client implementation doesn't return the ID for POSTs yet
		// We'll just say enqueued.
		success(fmt.Sprintf("Job enqueued: %s", cyan(jobType)))
	},
}

var jobRetryCmd = &cobra.Command{
	Use:   "retry <job-id>",
	Short: "Retry a failed job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobID := args[0]
		client := NewAPIClient()
		
		if err := client.Post(fmt.Sprintf("/api/queue/jobs/%s/retry", jobID), nil); err != nil {
			fail(fmt.Sprintf("Failed to retry: %v", err))
			return
		}

		success(fmt.Sprintf("Job %s queued for retry", cyan(jobID)))
	},
}

var jobStatusCmd = &cobra.Command{
	Use:   "status <job-id>",
	Short: "Get job status",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobID := args[0]
		
		client := NewAPIClient()
		var job struct {
			IDString  string      `json:"id"`
			Type      string      `json:"name"`
			Status    string      `json:"status"`
			Result    interface{} `json:"result,omitempty"`
			Error     string      `json:"error,omitempty"`
			Attempts  int         `json:"attempts"`
			CreatedAt time.Time   `json:"created_at"`
		}

		if err := client.Get(fmt.Sprintf("/api/queue/jobs/%s", jobID), &job); err != nil {
			fail(fmt.Sprintf("Failed to get status: %v", err))
			return
		}

		fmt.Println(bold("📋 Job Status"))
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID:\t%s\n", cyan(job.IDString))
		fmt.Fprintf(w, "Type:\t%s\n", job.Type)
		
		statusColor := green
		if job.Status == "failed" {
			statusColor = red
		} else if job.Status == "pending" {
			statusColor = yellow
		}
		fmt.Fprintf(w, "Status:\t%s\n", statusColor(job.Status))
		
		if job.Error != "" {
			fmt.Fprintf(w, "Error:\t%s\n", red(job.Error))
		}
		
		fmt.Fprintf(w, "Attempts:\t%d\n", job.Attempts)
		fmt.Fprintf(w, "Created:\t%s\n", job.CreatedAt.Format(time.RFC3339))
		w.Flush()
		
		if job.Result != nil {
			fmt.Println()
			fmt.Println(bold("Result:"))
			res, _ := json.MarshalIndent(job.Result, "", "  ")
			fmt.Println(string(res))
		}
	},
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
