# blowerboo

A minimal orchestration layer for AI media generation. Give it a prompt; get back structured, provider-ready payloads for images and video.

---

## What it does

blowerboo runs a three-agent pipeline:

1. **Spec Agent** — reads your raw prompt, identifies ambiguities, asks clarifying questions, and produces a structured creative spec (characters, environment, color palette, camera direction, output format, etc.).
2. **Planner Agent** — takes the spec and creates an ordered shot list. A single prompt may decompose into multiple shots (e.g. a title card, three scene shots, an outro).
3. **Execution Agent** — translates each shot into provider-ready API payloads and submits them to the configured media generation service (Kling, Runway, Midjourney, Leonardo, etc.).

---

## Workflow

```
you: "a lone astronaut on a red desert planet at dusk"
         │
         ▼
  [ Spec Agent ]
  - clarifying questions (style? aspect ratio? mood?)
  - structured spec: environment, camera, palette, format
         │
         ▼
  [ Planner Agent ]
  - shot list: [establishing wide shot, close-up visor reflection, pull-back reveal]
         │
         ▼
  [ Execution Agent ]
  - payload for each shot → Kling / Runway / Midjourney
  - returns job IDs and output URLs
         │
         ▼
  ./projects/<project-id>/results.json
```

---

## Directory Structure

```
blowerboo/
├── cmd/blowerboo/        # CLI entry point
├── internal/
│   ├── agents/spec/      # Spec Agent
│   ├── agents/planner/   # Planner Agent
│   ├── agents/execution/ # Execution Agent
│   ├── models/           # All shared data structs (DTOs)
│   ├── orchestrator/     # Pipeline coordinator
│   └── providers/        # Provider adapter interface + registry
├── projects/             # Per-run output (gitignored)
├── docs/                 # Architecture notes
├── .env.example
├── go.mod
└── README.md
```

---

## Quick Start

```bash
git clone https://github.com/blowerboo/blowerboo
cd blowerboo
cp .env.example .env
# fill in your API keys
# NOTE: CODEX_API_KEY is optional for now and reserved for a future
# Codex/OpenAI-backed implementation of Spec/Planner agents.

go run ./cmd/blowerboo "a lone astronaut on a red desert planet at dusk"
```

The pipeline runs with stub agents by default. Real LLM-backed agents and
provider adapters are added by implementing the interfaces in each `agents/`
package.

---

## Adding a Provider

Implement the `providers.Adapter` interface and register it in `main.go`:

```go
// In cmd/blowerboo/main.go
import "github.com/blowerboo/blowerboo/internal/providers/kling"

registry.Register(kling.New(os.Getenv("KLING_API_KEY")))
```

The adapter handles translating `ExecutionPayload` into Kling's native request
format. Nothing else in the pipeline needs to change.

---

## Future Roadmap

- [ ] Real LLM-backed Spec Agent (Anthropic Claude)
- [ ] Real LLM-backed Planner Agent
- [ ] Kling video adapter
- [ ] Runway video adapter
- [ ] Midjourney image adapter
- [ ] Leonardo image adapter
- [ ] Higgsfield adapter
- [ ] Flux adapter
- [ ] Stable Diffusion (local) adapter
- [ ] Async job polling loop
- [ ] Project state persistence to JSON files in `./projects/`
- [ ] HTTP API mode (serve the pipeline over REST)
- [ ] Web UI for reviewing specs and shots before executing
- [ ] Cost estimation before submission
- [ ] Reference image upload support
- [ ] Batch mode (multiple prompts from a file)
- [ ] Critic Agent (review and score generated assets)
- [ ] Versioned spec files
- [ ] Prompt template library
- [ ] Image/video analysis for iterative refinement

---

## Possible Integrations

| Provider | Type | Notes |
|---|---|---|
| Kling | Video | Strong motion, Chinese provider |
| Runway Gen-3 | Video | Industry standard |
| Midjourney | Image | Best aesthetic quality |
| Leonardo.Ai | Image | Fast, good for storyboards |
| Higgsfield | Video | Character-consistent generation |
| Flux | Image | Open weights, fast inference |
| Stable Diffusion | Image | Self-hosted option |
| Luma Dream Machine | Video | Photorealistic video |
| Pika Labs | Video | Motion control |
| ElevenLabs | Audio | Voiceover for video shots |
| Anthropic Claude | LLM | Spec + Planner agents |
| OpenAI GPT-4o | LLM | Alternate LLM backend |

---

## Contributing

Keep it simple. See `.github/copilot-instructions.md` for the design rules.
