package providers

import (
	"context"

	"github.com/blowerboo/blowerboo/internal/models"
)

// Adapter is the minimal interface every media provider must
// satisfy. Each concrete adapter (Kling, Runway, Midjourney,
// etc.) lives in its own file in this package.
type Adapter interface {
	// Name returns the canonical provider identifier,
	// e.g. "kling", "runway", "midjourney".
	Name() string

	// Supports reports whether the adapter can handle the
	// given payload's output format and parameters.
	Supports(payload models.ExecutionPayload) bool

	// Submit sends the payload to the provider and returns
	// an ExecutionResult. For async providers the result
	// status is "submitted" with a JobID; callers poll
	// separately via Status().
	Submit(ctx context.Context, payload models.ExecutionPayload) (models.ExecutionResult, error)

	// Status fetches the current state of a previously
	// submitted async job. Returns the same ExecutionResult
	// type with an updated status and OutputURL when ready.
	Status(ctx context.Context, jobID string) (models.ExecutionResult, error)
}

// Registry is a thin, in-process map of registered adapters.
// No reflection, no magic — just a named map.
type Registry struct {
	adapters map[string]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

// Register adds an adapter under its Name(). Panics on
// duplicate registration to catch wiring mistakes at startup.
func (r *Registry) Register(a Adapter) {
	name := a.Name()
	if _, exists := r.adapters[name]; exists {
		panic("providers: duplicate adapter registered: " + name)
	}
	r.adapters[name] = a
}

// Get retrieves an adapter by name. Returns nil, false when
// the name is not registered.
func (r *Registry) Get(name string) (Adapter, bool) {
	a, ok := r.adapters[name]
	return a, ok
}

// All returns every registered adapter, useful for capability
// checks during execution planning.
func (r *Registry) All() []Adapter {
	out := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, a)
	}
	return out
}
