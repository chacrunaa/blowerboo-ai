// Package orchestrator coordinates the three agents in
// sequence: raw prompt → spec (with optional Q&A) → plan →
// execution payloads → results.
//
// The orchestrator does NOT contain business logic. It is
// purely a call sequencer that passes data structures between
// agents and records state into a Project.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/blowerboo/blowerboo/internal/agents/execution"
	"github.com/blowerboo/blowerboo/internal/agents/planner"
	"github.com/blowerboo/blowerboo/internal/agents/spec"
	"github.com/blowerboo/blowerboo/internal/models"
	"github.com/blowerboo/blowerboo/internal/providers"
)

// AnswerFunc is called by the orchestrator when the spec agent
// returns clarifying questions. The caller provides answers
// (e.g. via CLI prompt, HTTP handler, or a test fixture).
// Return nil to abort the pipeline.
type AnswerFunc func(questions []models.ClarifyingQuestion) ([]models.ClarifyingAnswer, error)

// Orchestrator sequences the pipeline agents.
type Orchestrator struct {
	specAgent      spec.Agent
	plannerAgent   planner.Agent
	executionAgent execution.Agent
	registry       *providers.Registry

	// answerFn is how the orchestrator surfaces clarifying
	// questions to whoever is driving the pipeline. Callers
	// inject their own implementation (CLI, HTTP, test stub).
	answerFn AnswerFunc
}

// New constructs an Orchestrator. All dependencies are
// required; pass stub implementations during development.
func New(
	specAgent spec.Agent,
	plannerAgent planner.Agent,
	executionAgent execution.Agent,
	registry *providers.Registry,
	answerFn AnswerFunc,
) *Orchestrator {
	return &Orchestrator{
		specAgent:      specAgent,
		plannerAgent:   plannerAgent,
		executionAgent: executionAgent,
		registry:       registry,
		answerFn:       answerFn,
	}
}

// Run executes the full pipeline for the given raw prompt and
// returns a populated Project. The project records every
// intermediate artifact so callers can inspect or persist it.
func (o *Orchestrator) Run(ctx context.Context, rawPrompt models.RawPrompt) (models.Project, error) {
	project := models.Project{
		ID:        fmt.Sprintf("proj-%d", time.Now().UnixNano()),
		Prompt:    rawPrompt,
		Status:    "speccing",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// ── Step 1: Clarify ──────────────────────────────────────
	questions, err := o.specAgent.Clarify(ctx, rawPrompt)
	if err != nil {
		return project, fmt.Errorf("spec clarify: %w", err)
	}

	var answers []models.ClarifyingAnswer
	if len(questions) > 0 {
		project.Questions = questions
		project.UpdatedAt = time.Now()

		if o.answerFn == nil {
			return project, fmt.Errorf("spec agent returned %d questions but no AnswerFunc is configured", len(questions))
		}

		answers, err = o.answerFn(questions)
		if err != nil {
			return project, fmt.Errorf("answering clarifying questions: %w", err)
		}
		project.Answers = answers
		project.UpdatedAt = time.Now()
	}

	// ── Step 2: Build Spec ───────────────────────────────────
	builtSpec, err := o.specAgent.Build(ctx, rawPrompt, answers)
	if err != nil {
		return project, fmt.Errorf("spec build: %w", err)
	}
	project.Spec = &builtSpec
	project.Status = "planning"
	project.UpdatedAt = time.Now()

	// ── Step 3: Plan ─────────────────────────────────────────
	plan, err := o.plannerAgent.Plan(ctx, builtSpec)
	if err != nil {
		return project, fmt.Errorf("planner: %w", err)
	}
	project.Plan = &plan
	project.Status = "executing"
	project.UpdatedAt = time.Now()

	// ── Step 4: Format payloads ──────────────────────────────
	payloads, err := o.executionAgent.Format(ctx, plan, builtSpec, o.registry)
	if err != nil {
		return project, fmt.Errorf("execution format: %w", err)
	}

	// ── Step 5: Submit ───────────────────────────────────────
	results, err := o.executionAgent.Submit(ctx, payloads, o.registry)
	if err != nil {
		return project, fmt.Errorf("execution submit: %w", err)
	}
	project.Results = results
	project.Status = "done"
	project.UpdatedAt = time.Now()

	return project, nil
}
