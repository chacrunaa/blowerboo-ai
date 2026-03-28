// Package spec implements the Spec Agent, which transforms a
// raw user prompt into a structured Spec. When the prompt is
// ambiguous the agent returns clarifying questions instead of
// proceeding; the orchestrator re-invokes it once answers are
// supplied.
package spec

import (
	"context"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

// Agent is the interface the orchestrator calls.
// Keeping it local to this package means the orchestrator
// depends on this interface, not on a shared mega-interface.
type Agent interface {
	// Clarify inspects the prompt and returns any questions
	// the agent needs answered before it can produce a Spec.
	// Returns an empty slice when the prompt is clear enough.
	Clarify(ctx context.Context, prompt models.RawPrompt) ([]models.ClarifyingQuestion, error)

	// Build produces a structured Spec from the prompt and
	// any answers provided to prior clarifying questions.
	Build(ctx context.Context, prompt models.RawPrompt, answers []models.ClarifyingAnswer) (models.Spec, error)
}

// stubAgent is the initial no-op implementation used during
// development. Replace with an LLM-backed implementation
// without changing any call sites.
type stubAgent struct{}

// New returns a stub Agent. Swap the return type for a real
// implementation once the LLM wiring is ready.
func New() Agent {
	return &stubAgent{}
}

func (a *stubAgent) Clarify(_ context.Context, _ models.RawPrompt) ([]models.ClarifyingQuestion, error) {
	// Stub: assume prompts are always clear enough.
	// A real implementation calls an LLM and parses structured
	// output to determine ambiguity.
	return nil, nil
}

func (a *stubAgent) Build(_ context.Context, prompt models.RawPrompt, _ []models.ClarifyingAnswer) (models.Spec, error) {
	// Stub: echo back a minimal Spec populated with the raw text.
	return models.Spec{
		ID:           "spec-stub-001",
		PromptID:     prompt.ID,
		Narrative:    prompt.Text,
		OutputFormat: "image",
		AspectRatio:  "16:9",
		CreatedAt:    time.Now(),
	}, nil
}
