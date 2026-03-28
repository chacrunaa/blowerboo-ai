# Copilot Instructions for blowerboo

These instructions define how Copilot should assist on this codebase.
Follow them closely to keep the project minimal, readable, and easy to grow.

---

## Core Philosophy

This is a small orchestration project. The goal is **clarity over cleverness**.
Write code a new contributor can understand in five minutes without a diagram.

---

## Architecture Rules

### 1. Do not create packages prematurely

Only create a new package when:
- It has more than one file of related logic AND
- The logic is genuinely reused across two or more other packages.

A single-file concept does NOT need its own package. Add it to the closest
existing package first.

### 2. Do not introduce abstractions without a concrete reason

No interfaces without at least two implementations or a testing requirement.
No wrapper types around primitives unless the type carries real invariants.
No generic type parameters unless you have three or more concrete use cases.

### 3. Keep the agent packages thin

Each `internal/agents/<name>/agent.go` contains:
- One interface
- One concrete struct implementing it
- A `New()` constructor

Do not add routing logic, retry loops, or rate limiting inside agents.
Those belong in the orchestrator or in provider adapters.

### 4. The orchestrator is a sequencer, not a brain

`orchestrator.Run()` calls agents in order and pipes data between them.
It does NOT contain prompt logic, API calls, or output formatting.
If you find yourself putting complex logic in the orchestrator, it belongs
in one of the agents or a provider adapter instead.

### 5. models.go is the source of truth for data shapes

All shared structs live in `internal/models/models.go`. Do not duplicate
struct definitions. Do not add methods to model structs that contain
business logic — keep models as plain data containers (DTOs).

### 6. Provider adapters are isolated

Each provider adapter (Kling, Runway, etc.) lives in its own file under
`internal/providers/`. It implements `providers.Adapter` and knows nothing
about agents or the orchestrator. The only data it handles is
`models.ExecutionPayload` in and `models.ExecutionResult` out.

---

## Code Style

- Use plain `struct` types and named fields. Avoid embedding unless it is
  genuinely a composition (not a mixin workaround).
- Return errors explicitly. Do not panic except in `New()` wiring checks
  (e.g., duplicate registry entries at startup).
- Use `context.Context` as the first argument on all functions that may
  call external services.
- Prefer clear, full variable names over short abbreviations. `spec` not `s`.
  `payload` not `p` (except in short closures).
- Group related fields in structs with blank-line separated comment blocks
  rather than splitting into separate structs prematurely.
- Write table-driven tests for agents using stub inputs.

---

## What NOT to do

- Do NOT add an event bus, message queue, or pub/sub system.
- Do NOT implement CQRS, saga patterns, or event sourcing.
- Do NOT create a dependency injection container.
- Do NOT add global state (global vars, init() side effects).
- Do NOT use `interface{}` or `any` as a general-purpose escape hatch.
  Use it only in `ProviderParams map[string]any` for provider-specific
  overrides where the schema is truly variable.
- Do NOT split `models.go` into many files before there are more than ~200
  lines of model definitions.
- Do NOT add a database or ORM. JSON files in `./projects/` are sufficient
  for persistence until requirements clearly demand more.

---

## Grow the project in this order

1. Replace stub agents with real LLM-backed implementations one at a time.
2. Add one real provider adapter (start with whichever you have API access to).
3. Add JSON persistence for Project state in `./projects/`.
4. Add async polling once a real provider needs it.
5. Only then consider an HTTP API layer if interactive use demands it.

Keep the diff small. Ship working stubs before wiring real backends.
