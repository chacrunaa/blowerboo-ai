package models

import "time"

// ============================================================
// Input
// ============================================================

// RawPrompt is the unprocessed user input that starts the pipeline.
type RawPrompt struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Spec Agent output
// ============================================================

// ClarifyingQuestion is returned by the spec agent when the
// prompt is ambiguous. The orchestrator surfaces these to the
// caller before continuing.
type ClarifyingQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	// Optional hint shown to the user alongside the question.
	Hint string `json:"hint,omitempty"`
}

// ClarifyingAnswer pairs a user-provided answer with the
// original question ID.
type ClarifyingAnswer struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

// Spec is the structured, validated output of the spec agent.
// All fields are optional slices/strings so the agent can
// populate only what the prompt supplies.
type Spec struct {
	ID       string `json:"id"`
	PromptID string `json:"prompt_id"`

	// Narrative / content
	Characters  []string `json:"characters,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Mood        string   `json:"mood,omitempty"`
	Narrative   string   `json:"narrative,omitempty"`

	// Visual direction
	Style        string   `json:"style,omitempty"`
	ColorPalette []string `json:"color_palette,omitempty"`
	Lighting     string   `json:"lighting,omitempty"`
	References   []string `json:"references,omitempty"` // URLs or descriptions

	// Camera & motion
	CameraAngle     string `json:"camera_angle,omitempty"`
	CameraMotion    string `json:"camera_motion,omitempty"`
	MotionIntensity string `json:"motion_intensity,omitempty"` // e.g. "subtle", "dynamic"

	// Output
	OutputFormat string `json:"output_format"`           // "image" | "video" | "sequence"
	AspectRatio  string `json:"aspect_ratio,omitempty"`  // e.g. "16:9", "9:16", "1:1"
	DurationSec  int    `json:"duration_sec,omitempty"`  // for video

	// Restrictions / negative prompts
	Restrictions []string `json:"restrictions,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Planner Agent output
// ============================================================

// Shot represents a single atomic media generation unit.
// A plan is composed of one or more shots.
type Shot struct {
	ID          string `json:"id"`
	Order       int    `json:"order"`
	Description string `json:"description"`

	// Each shot inherits or overrides from the parent Spec.
	Style        string   `json:"style,omitempty"`
	CameraAngle  string   `json:"camera_angle,omitempty"`
	CameraMotion string   `json:"camera_motion,omitempty"`
	DurationSec  int      `json:"duration_sec,omitempty"`
	Tags         []string `json:"tags,omitempty"`

	// Which provider(s) are preferred for this shot.
	// Empty means "any compatible provider".
	PreferredProviders []string `json:"preferred_providers,omitempty"`
}

// Plan is the structured execution blueprint produced by the
// planner agent. It contains an ordered list of shots and any
// cross-shot notes.
type Plan struct {
	ID     string `json:"id"`
	SpecID string `json:"spec_id"`

	Shots []Shot `json:"shots"`
	Notes string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Execution Agent output
// ============================================================

// ExecutionPayload is the provider-agnostic envelope the
// execution agent produces for each shot. The provider adapter
// translates this into the provider's native API request.
type ExecutionPayload struct {
	ShotID   string `json:"shot_id"`
	Provider string `json:"provider"` // e.g. "kling", "runway", "midjourney"

	// Resolved prompt text ready for the provider.
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Provider-agnostic parameters
	AspectRatio string `json:"aspect_ratio,omitempty"`
	DurationSec int    `json:"duration_sec,omitempty"`
	Style       string `json:"style,omitempty"`

	// Escape hatch: provider-specific overrides that don't fit
	// the generic fields above. The adapter reads from this map.
	ProviderParams map[string]any `json:"provider_params,omitempty"`
}

// ExecutionResult captures what came back from the provider
// after submitting a payload.
type ExecutionResult struct {
	ShotID   string `json:"shot_id"`
	Provider string `json:"provider"`

	// JobID is the async job reference returned by the provider.
	// Empty when the result is synchronous.
	JobID  string `json:"job_id,omitempty"`
	Status string `json:"status"` // "submitted" | "completed" | "failed"

	// OutputURL is populated once the asset is ready.
	OutputURL string    `json:"output_url,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Pipeline envelope
// ============================================================

// Project is the top-level container that tracks a single
// end-to-end run of the pipeline. Useful for persisting state.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`

	Prompt  RawPrompt        `json:"prompt"`
	Spec    *Spec            `json:"spec,omitempty"`
	Plan    *Plan            `json:"plan,omitempty"`
	Results []ExecutionResult `json:"results,omitempty"`

	// Questions asked during spec phase and their answers.
	Questions []ClarifyingQuestion `json:"questions,omitempty"`
	Answers   []ClarifyingAnswer   `json:"answers,omitempty"`

	Status    string    `json:"status"` // "speccing" | "planning" | "executing" | "done" | "failed"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
