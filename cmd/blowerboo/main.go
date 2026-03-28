// Command blowerboo is the CLI entry point for the media
// generation orchestration pipeline.
//
// Usage:
//
//	blowerboo "a lone astronaut on a red desert planet at dusk"
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blowerboo/blowerboo/internal/agents/execution"
	"github.com/blowerboo/blowerboo/internal/agents/planner"
	"github.com/blowerboo/blowerboo/internal/agents/spec"
	"github.com/blowerboo/blowerboo/internal/models"
	"github.com/blowerboo/blowerboo/internal/orchestrator"
	"github.com/blowerboo/blowerboo/internal/providers"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: blowerboo <prompt>")
		os.Exit(1)
	}

	promptText := strings.Join(os.Args[1:], " ")

	rawPrompt := models.RawPrompt{
		ID:        fmt.Sprintf("prompt-%d", time.Now().UnixNano()),
		Text:      promptText,
		CreatedAt: time.Now(),
	}

	// Wire up agents (stubs until real LLM backends are added).
	specAgent := spec.New()
	plannerAgent := planner.New()
	executionAgent := execution.New()

	// Provider registry — empty until real adapters are registered.
	registry := providers.NewRegistry()

	// answerFn reads answers from stdin for the CLI use-case.
	answerFn := func(questions []models.ClarifyingQuestion) ([]models.ClarifyingAnswer, error) {
		scanner := bufio.NewScanner(os.Stdin)
		answers := make([]models.ClarifyingAnswer, 0, len(questions))
		for _, q := range questions {
			fmt.Printf("\nQuestion: %s\n", q.Question)
			if q.Hint != "" {
				fmt.Printf("Hint: %s\n", q.Hint)
			}
			fmt.Print("Answer: ")
			if scanner.Scan() {
				answers = append(answers, models.ClarifyingAnswer{
					QuestionID: q.ID,
					Answer:     scanner.Text(),
				})
			}
		}
		return answers, scanner.Err()
	}

	o := orchestrator.New(specAgent, plannerAgent, executionAgent, registry, answerFn)

	ctx := context.Background()
	project, err := o.Run(ctx, rawPrompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipeline error: %v\n", err)
		os.Exit(1)
	}

	// Pretty-print the project to stdout.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(project); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		os.Exit(1)
	}
}
