// Package execution implements the Execution Agent, which
// translates a Plan into provider-ready ExecutionPayloads and
// optionally submits them via the provider registry.
package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
	"github.com/blowerboo/blowerboo/internal/providers"
)

// Agent is the interface the orchestrator calls.
type Agent interface {
	// Format converts each Shot in the plan into an
	// ExecutionPayload targeted at a specific provider.
	// If no preferred provider is listed on the shot the
	// agent picks the first registered compatible adapter.
	Format(ctx context.Context, plan models.Plan, spec models.Spec, registry *providers.Registry) ([]models.ExecutionPayload, error)

	// Submit sends all payloads and collects results.
	// This is a separate step so callers can inspect payloads
	// before committing API calls.
	Submit(ctx context.Context, payloads []models.ExecutionPayload, registry *providers.Registry) ([]models.ExecutionResult, error)
}

type stubAgent struct{}

// New returns a stub Agent.
func New() Agent {
	return &stubAgent{}
}

func (a *stubAgent) Format(_ context.Context, plan models.Plan, spec models.Spec, _ *providers.Registry) ([]models.ExecutionPayload, error) {
	payloads := make([]models.ExecutionPayload, 0, len(plan.Shots))
	for _, shot := range plan.Shots {
		payloads = append(payloads, models.ExecutionPayload{
			ShotID:      shot.ID,
			Provider:    "stub",
			Prompt:      shot.Description,
			AspectRatio: spec.AspectRatio,
			DurationSec: shot.DurationSec,
			Style:       shot.Style,
		})
	}
	return payloads, nil
}

func (a *stubAgent) Submit(ctx context.Context, payloads []models.ExecutionPayload, registry *providers.Registry) ([]models.ExecutionResult, error) {
	results := make([]models.ExecutionResult, 0, len(payloads))
	for _, p := range payloads {
		adapter, ok := registry.Get(p.Provider)
		if !ok {
			// Stub fallback: record as submitted without a real call.
			results = append(results, models.ExecutionResult{
				ShotID:    p.ShotID,
				Provider:  p.Provider,
				Status:    "submitted",
				JobID:     fmt.Sprintf("stub-job-%s", p.ShotID),
				CreatedAt: time.Now(),
			})
			continue
		}
		// Real adapter path — used once real providers are wired in.
		result, err := adapter.Submit(ctx, p)
		if err != nil {
			results = append(results, models.ExecutionResult{
				ShotID:    p.ShotID,
				Provider:  p.Provider,
				Status:    "failed",
				Error:     err.Error(),
				CreatedAt: time.Now(),
			})
			continue
		}
		results = append(results, result)
	}
	return results, nil
}
