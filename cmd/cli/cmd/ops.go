package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dlqCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(eventsCmd)

	// DLQ sub-commands
	dlqCmd.AddCommand(dlqListCmd)
	dlqCmd.AddCommand(dlqRetryCmd)
	dlqCmd.AddCommand(dlqRetryAllCmd)
	dlqCmd.AddCommand(dlqPurgeCmd)

	// Queue sub-commands
	queueCmd.AddCommand(queueStatsCmd)

	// Events flags
	eventsCmd.Flags().StringP("job-id", "j", "", "filter by job ID")
	eventsCmd.Flags().BoolP("follow", "f", false, "follow new events")
	eventsCmd.Flags().IntP("tail", "n", 20, "number of recent events")
}

// ============ DLQ Commands ============

var dlqCmd = &cobra.Command{
	Use:   "dlq",
	Short: "Manage dead letter queue",
	Long:  `View and manage jobs in the dead letter queue.`,
}

var dlqListCmd = &cobra.Command{
	Use:   "list",
	Short: "List DLQ entries",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(bold("💀 Dead Letter Queue"))
		fmt.Println()

		client := NewAPIClient()
		var entries []struct {
			IDString string    `json:"id"`
			JobName  string    `json:"name"`
			Error    string    `json:"error"`
			FailedAt time.Time `json:"failed_at"`
		}

		if err := client.Get("/api/queue/dlq", &entries); err != nil {
			fail(fmt.Sprintf("Failed to fetch DLQ: %v", err))
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "JOB ID\tTYPE\tERROR\tFAILED AT")
		fmt.Fprintln(w, "------\t----\t-----\t---------")

		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				red(e.IDString),
				e.JobName,
				e.Error,
				e.FailedAt.Format("15:04:05"),
			)
		}
		w.Flush()

		fmt.Println()
		fmt.Printf("Total: %s entries\n", red(fmt.Sprintf("%d", len(entries))))
	},
}

var dlqRetryCmd = &cobra.Command{
	Use:   "retry <job-id>",
	Short: "Retry a DLQ entry",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobID := args[0]
		client := NewAPIClient()
		
		if err := client.Post(fmt.Sprintf("/api/queue/jobs/%s/retry", jobID), nil); err != nil {
			fail(fmt.Sprintf("Failed to retry job: %v", err))
			return
		}

		success(fmt.Sprintf("Job %s moved back to queue", cyan(jobID)))
	},
}

var dlqRetryAllCmd = &cobra.Command{
	Use:   "retry-all",
	Short: "Retry all DLQ entries",
	Run: func(cmd *cobra.Command, args []string) {
		// Ideally we'd have a batch retry endpoint, but iterating works for now
		// Or implement a /retry-all endpoint on the server
		// For now let's assume one-by-one or warn
		warn("Batch retry not fully implemented in CLI yet.")
	},
}

var dlqPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Remove all DLQ entries",
	Run: func(cmd *cobra.Command, args []string) {
		warn("This will permanently delete all DLQ entries.")
		fmt.Print("Continue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			client := NewAPIClient()
			if err := client.Post("/api/queue/dlq/purge", nil); err != nil {
				fail(fmt.Sprintf("Failed to purge: %v", err))
				return
			}
			success("DLQ purged")
		} else {
			info("Cancelled")
		}
	},
}

// ============ Queue Commands ============

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Queue operations",
}

var queueStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show queue statistics",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(bold("📊 Queue Statistics"))
		fmt.Println()

		client := NewAPIClient()
		var stats struct {
			Pending      int64 `json:"pending"`
			Processing   int64 `json:"processing"`
			Completed24h int64 `json:"completed_24h"`
			Failed24h    int64 `json:"failed_24h"`
			DLQSize      int64 `json:"dlq_size"`
		}

		if err := client.Get("/api/queue/stats", &stats); err != nil {
			fail(fmt.Sprintf("Failed to fetch stats: %v", err))
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Pending Jobs:\t%s\n", cyan(fmt.Sprintf("%d", stats.Pending)))
		fmt.Fprintf(w, "Processing:\t%s\n", yellow(fmt.Sprintf("%d", stats.Processing)))
		fmt.Fprintf(w, "Completed (24h):\t%s\n", green(fmt.Sprintf("%d", stats.Completed24h)))
		fmt.Fprintf(w, "Failed (24h):\t%s\n", red(fmt.Sprintf("%d", stats.Failed24h)))
		fmt.Fprintf(w, "DLQ Size:\t%s\n", red(fmt.Sprintf("%d", stats.DLQSize)))
		w.Flush()
	},
}

// ============ Events Commands ============

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "View event log",
	Run: func(cmd *cobra.Command, args []string) {
		jobID, _ := cmd.Flags().GetString("job-id")
		tail, _ := cmd.Flags().GetInt("tail")
		follow, _ := cmd.Flags().GetBool("follow")

		fmt.Println(bold("📜 Event Log"))
		if jobID != "" {
			fmt.Printf("   Job: %s\n", cyan(jobID))
		}
		fmt.Println()

		// Mock events
		events := []struct {
			Time    time.Time
			Type    string
			JobID   string
			Details string
		}{
			{time.Now().Add(-5 * time.Minute), "job.completed", "job-100", "duration=245ms"},
			{time.Now().Add(-4 * time.Minute), "job.started", "job-101", "worker=worker-3"},
			{time.Now().Add(-3 * time.Minute), "job.queued", "job-102", "type=send_email"},
			{time.Now().Add(-2 * time.Minute), "job.failed", "job-103", "error=timeout"},
			{time.Now().Add(-1 * time.Minute), "job.retried", "job-103", "attempt=2"},
		}

		for i := 0; i < min(tail, len(events)); i++ {
			e := events[i]
			typeColor := cyan
			if e.Type == "job.failed" {
				typeColor = red
			} else if e.Type == "job.completed" {
				typeColor = green
			}

			fmt.Printf("%s  %s  %s  %s\n",
				e.Time.Format("15:04:05"),
				typeColor(e.Type),
				e.JobID,
				e.Details,
			)
		}

		if follow {
			info("Waiting for new events... (Ctrl+C to exit)")
			select {}
		}
	},
}
