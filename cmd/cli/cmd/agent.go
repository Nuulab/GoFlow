package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nuulab/goflow/pkg/agent"
	"github.com/nuulab/goflow/pkg/core"
	"github.com/nuulab/goflow/pkg/llm/anthropic"
	"github.com/nuulab/goflow/pkg/llm/gemini"
	"github.com/nuulab/goflow/pkg/llm/openai"
	"github.com/nuulab/goflow/pkg/tools"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(agentCmd)

	// Flags
	agentCmd.Flags().StringSliceP("tools", "t", []string{}, "tools to enable")
	agentCmd.Flags().IntP("max-iterations", "m", 10, "maximum iterations")
	agentCmd.Flags().BoolP("interactive", "i", false, "interactive mode")
	agentCmd.Flags().String("model", "", "LLM model to use")
	agentCmd.Flags().String("provider", "", "LLM provider (openai, anthropic, gemini)")
}

var agentCmd = &cobra.Command{
	Use:   "agent [task]",
	Short: "Run an AI agent",
	Long:  `Run a GoFlow AI agent with the specified task.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Flags().Lookup("tools") // Ignore for now
		maxIter, _ := cmd.Flags().GetInt("max-iterations")
		interactive, _ := cmd.Flags().GetBool("interactive")
		model, _ := cmd.Flags().GetString("model")
		provider, _ := cmd.Flags().GetString("provider")

		// Create LLM
		llm, err := createLLM(provider, model)
		if err != nil {
			fail(err.Error())
			os.Exit(1)
		}

		// Create tools
		agentTools := tools.BuiltinTools()
		// TODO: Add dynamic tools if needed

		// Create Agent
		a := agent.New(llm, agentTools, agent.WithMaxIterations(maxIter))

		if interactive {
			runInteractiveAgent(a)
			return
		}

		if len(args) == 0 {
			fail("Task required. Use -i for interactive mode.")
			return
		}

		task := strings.Join(args, " ")
		runAgentTask(a, task)
	},
}

func createLLM(provider, model string) (core.LLM, error) {
	if provider == "" {
		provider = viper.GetString("llm.provider")
		if provider == "" {
			provider = "openai" // Default
		}
	}

	if model == "" {
		model = viper.GetString("llm.model")
	}

	key := viper.GetString("llm.api_key")

	switch provider {
	case "openai":
		opts := []openai.Option{}
		if model != "" {
			opts = append(opts, openai.WithModel(model))
		}
		return openai.New(key, opts...), nil
	case "anthropic":
		opts := []anthropic.Option{}
		if model != "" {
			opts = append(opts, anthropic.WithModel(model))
		}
		return anthropic.New(key, opts...), nil
	case "gemini":
		opts := []gemini.Option{}
		if model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		return gemini.New(key, opts...), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func runAgentTask(a *agent.Agent, task string) {
	fmt.Println(bold("🤖 Running Agent"))
	fmt.Printf("   Task: %s\n", cyan(task))
	fmt.Println()

	fmt.Println(yellow("Thinking..."))
	start := time.Now()

	result, err := a.Run(context.Background(), task)
	if err != nil {
		fail(fmt.Sprintf("Agent failed: %v", err))
		return
	}

	duration := time.Since(start)
	fmt.Println()
	fmt.Printf("%s Completed in %s\n", green("✓"), duration.Round(time.Millisecond))
	fmt.Println()

	fmt.Println(bold("Result:"))
	fmt.Println(result.Output)
	fmt.Println()

	if len(result.ToolCalls) > 0 {
		fmt.Println(bold("Tools Used:"))
		for i, tc := range result.ToolCalls {
			fmt.Printf("  %d. %s(%s) -> %s\n", i+1, cyan(tc.Name), tc.Input, tc.Output)
		}
	}
}

func runInteractiveAgent(a *agent.Agent) {
	fmt.Println(bold("🤖 GoFlow Agent - Interactive Mode"))
	fmt.Println("Type your task and press Enter. Type 'exit' to quit.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(cyan("You: "))
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			info("Goodbye!")
			break
		}

		fmt.Println()
		fmt.Println(yellow("Thinking..."))
		
		result, err := a.Run(context.Background(), input)
		if err != nil {
			fail(fmt.Sprintf("Error: %v", err))
			continue
		}

		fmt.Println()
		fmt.Printf(green("Agent: ") + result.Output)
		fmt.Println()
		fmt.Println()
	}
}
