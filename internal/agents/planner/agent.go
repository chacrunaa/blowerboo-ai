// Package planner implements the Planner Agent, which converts
// a structured Spec into an ordered shot list (Plan).
package planner

import (
	"context"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

// Agent is the interface the orchestrator calls.
type Agent interface {
	// Plan takes a finalized Spec and returns a Plan with an
	// ordered list of Shots.
	Plan(ctx context.Context, spec models.Spec) (models.Plan, error)
}

type stubAgent struct{}

// New returns a stub Agent.
func New() Agent {
	return &stubAgent{}
}

func (a *stubAgent) Plan(_ context.Context, spec models.Spec) (models.Plan, error) {
	// Stub: produce a single-shot plan from the spec.
	return models.Plan{
		ID:     "plan-stub-001",
		SpecID: spec.ID,
		Shots: []models.Shot{
			{
				ID:          "shot-001",
				Order:       1,
				Description: spec.Narrative,
				Style:       spec.Style,
				CameraAngle: spec.CameraAngle,
				DurationSec: spec.DurationSec,
			},
		},
		Notes:     "Single-shot stub plan.",
		CreatedAt: time.Now(),
	}, nil
}
