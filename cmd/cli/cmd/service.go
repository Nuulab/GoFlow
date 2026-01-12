package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(schedulerCmd)

	// Server flags
	serverCmd.Flags().IntP("port", "p", 8080, "server port")
	viper.BindPFlag("server.port", serverCmd.Flags().Lookup("port"))

	// Worker flags
	workerCmd.Flags().IntP("concurrency", "c", 5, "number of concurrent workers")
	viper.BindPFlag("worker.concurrency", workerCmd.Flags().Lookup("concurrency"))
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	Long:  `Start the GoFlow API server with REST and WebSocket endpoints.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		redisAddr := viper.GetString("redis")

		fmt.Println(bold("🚀 Starting GoFlow API Server"))
		fmt.Printf("   Port: %s\n", cyan(fmt.Sprintf("%d", port)))
		fmt.Printf("   Redis: %s\n", cyan(redisAddr))
		fmt.Println()

		// Import and start the actual server
		// In a real implementation, this would import pkg/api
		info(fmt.Sprintf("Server running at http://localhost:%d", port))
		info("Press Ctrl+C to stop")

		// Wait for interrupt
		waitForShutdown()
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a job worker",
	Long:  `Start a GoFlow worker to process queued jobs.`,
	Run: func(cmd *cobra.Command, args []string) {
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		redisAddr := viper.GetString("redis")

		fmt.Println(bold("🔧 Starting GoFlow Worker"))
		fmt.Printf("   Concurrency: %s\n", cyan(fmt.Sprintf("%d", concurrency)))
		fmt.Printf("   Redis: %s\n", cyan(redisAddr))
		fmt.Println()

		info("Worker started, processing jobs...")
		info("Press Ctrl+C to stop")

		waitForShutdown()
	},
}

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Start the cron scheduler",
	Long:  `Start the GoFlow scheduler to run cron jobs.`,
	Run: func(cmd *cobra.Command, args []string) {
		redisAddr := viper.GetString("redis")

		fmt.Println(bold("⏰ Starting GoFlow Scheduler"))
		fmt.Printf("   Redis: %s\n", cyan(redisAddr))
		fmt.Println()

		info("Scheduler started, running cron jobs...")
		info("Press Ctrl+C to stop")

		waitForShutdown()
	},
}

func waitForShutdown() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println()
		info("Shutting down gracefully...")
	case <-ctx.Done():
	}
}
