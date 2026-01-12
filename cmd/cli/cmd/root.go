// Package cmd provides the CLI commands for GoFlow.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "goflow",
	Short: "GoFlow - AI Orchestration Framework",
	Long: `
   ____       _____ _               
  / ___| ___ |  ___| | _____      __
 | |  _ / _ \| |_  | |/ _ \ \ /\ / /
 | |_| | (_) |  _| | | (_) \ V  V / 
  \____|\___/|_|   |_|\___/ \_/\_/  

GoFlow is an AI orchestration framework for building
intelligent agents, workflows, and pipelines.

Run 'goflow help <command>' for details on any command.
`,
	Version: "1.0.0",
}

// Execute runs the CLI.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./goflow.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().String("redis", "localhost:6379", "Redis/DragonflyDB address")

	// Bind flags to viper
	viper.BindPFlag("redis", rootCmd.PersistentFlags().Lookup("redis"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("goflow")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.goflow")
	}

	viper.SetEnvPrefix("GOFLOW")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Println("Using config:", viper.ConfigFileUsed())
	}
}

// Color helpers
func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }
func bold(s string) string   { return "\033[1m" + s + "\033[0m" }

func success(msg string) { fmt.Println(green("✓ ") + msg) }
func fail(msg string)    { fmt.Fprintln(os.Stderr, red("✗ ")+msg) }
func info(msg string)    { fmt.Println(cyan("ℹ ") + msg) }
func warn(msg string)    { fmt.Println(yellow("⚠ ") + msg) }
