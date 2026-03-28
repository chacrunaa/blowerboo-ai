# Architecture

## Pipeline

```
RawPrompt
    │
    ▼
SpecAgent.Clarify()       ← may surface ClarifyingQuestions to caller
    │
    ▼ (answers injected)
SpecAgent.Build()         → Spec
    │
    ▼
PlannerAgent.Plan()       → Plan (ordered []Shot)
    │
    ▼
ExecutionAgent.Format()   → []ExecutionPayload
    │
    ▼
ExecutionAgent.Submit()   → []ExecutionResult
    │
    ▼
Project (persisted)
```

## Key Design Decisions

### No shared Agent interface
Each agent package exports its own interface. This prevents the temptation of
a bloated shared interface that all agents must satisfy even when a method
makes no sense for them.

### Orchestrator is a concrete struct, not an interface
The orchestrator is constructed once in main.go and wired together. Interfaces
are introduced only if alternate orchestration strategies (batch, streaming,
async) are needed. Premature interface extraction here would just add noise.

### AnswerFunc for Q&A
The spec agent may ask questions. The orchestrator doesn't know whether it's
running in a CLI, HTTP server, or test harness, so it delegates to an
injected function. This keeps each layer testable without building a full
integration harness.

### ExecutionPayload as a translation boundary
The execution agent produces a provider-agnostic payload. Each provider
adapter is responsible for translating that into its own API shape. This
means the spec and planner agents never need to know which provider will
handle a shot.

### Registry over factory
The provider registry is a flat map. Adding a new provider is one line in
main.go: `registry.Register(mykling.New(apiKey))`. No factory patterns,
no reflection, no plugin system.
