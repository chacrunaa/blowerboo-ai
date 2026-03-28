// Пакет `orchestrator` координирует трех агентов в
// последовательности: сырой промпт -> спецификация (с опциональными уточнениями) -> план ->
// payload-ы выполнения -> результаты.
//
// Оркестратор НЕ содержит бизнес-логику. Это
// только секвенсор вызовов, который передает структуры данных между
// агентами и записывает состояние в `Project`.
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

// `AnswerFunc` вызывается оркестратором, когда spec-агент
// возвращает уточняющие вопросы. Вызывающая сторона передает ответы
// (например, через CLI-промпт, HTTP-обработчик или тестовую фикстуру).
// Верните `nil`, чтобы прервать пайплайн.
type AnswerFunc func(questions []models.ClarifyingQuestion) ([]models.ClarifyingAnswer, error)

// `Orchestrator` задает последовательность агентов пайплайна.
type Orchestrator struct {
	specAgent      spec.Agent
	plannerAgent   planner.Agent
	executionAgent execution.Agent
	registry       *providers.Registry

	// `answerFn` - способ, которым оркестратор отдает уточняющие
	// вопросы тому, кто запускает пайплайн. Вызывающие стороны
	// подставляют свою реализацию (CLI, HTTP, тестовая заглушка).
	answerFn AnswerFunc

	// PollInterval — пауза между запросами статуса асинхронных задач.
	// Ноль означает использование значения по умолчанию (5 секунд).
	PollInterval time.Duration

	// PollTimeout — максимальное суммарное время ожидания всех задач.
	// Ноль означает использование значения по умолчанию (10 минут).
	PollTimeout time.Duration
}

// `New` создает `Orchestrator`. Все зависимости
// обязательны; в разработке передавайте заглушки.
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

// `Run` выполняет полный пайплайн для заданного сырого промпта и
// возвращает заполненный `Project`. Проект хранит все
// промежуточные артефакты, чтобы их можно было просмотреть или сохранить.
func (o *Orchestrator) Run(ctx context.Context, rawPrompt models.RawPrompt) (models.Project, error) {
	project := models.Project{
		ID:        fmt.Sprintf("proj-%d", time.Now().UnixNano()),
		Prompt:    rawPrompt,
		Status:    "speccing",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// -- Шаг 1: Уточнение ---------------------------------------
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

	// -- Шаг 2: Построение спецификации --------------------------
	builtSpec, err := o.specAgent.Build(ctx, rawPrompt, answers)
	if err != nil {
		return project, fmt.Errorf("spec build: %w", err)
	}
	project.Spec = &builtSpec
	project.Status = "planning"
	project.UpdatedAt = time.Now()

	// -- Шаг 3: Планирование ------------------------------------
	plan, err := o.plannerAgent.Plan(ctx, builtSpec)
	if err != nil {
		return project, fmt.Errorf("planner: %w", err)
	}
	project.Plan = &plan
	project.Status = "executing"
	project.UpdatedAt = time.Now()

	// -- Шаг 4: Формирование payload-ов -------------------------
	payloads, err := o.executionAgent.Format(ctx, plan, builtSpec, o.registry)
	if err != nil {
		return project, fmt.Errorf("execution format: %w", err)
	}

	// -- Шаг 5: Отправка ----------------------------------------
	results, err := o.executionAgent.Submit(ctx, payloads, o.registry)
	if err != nil {
		return project, fmt.Errorf("execution submit: %w", err)
	}

	// -- Шаг 6: Polling до получения output_url ------------------
	results, err = o.pollUntilDone(ctx, results)
	if err != nil {
		return project, fmt.Errorf("polling: %w", err)
	}

	project.Results = results
	project.Status = "done"
	project.UpdatedAt = time.Now()

	return project, nil
}

// pollUntilDone периодически запрашивает статус задач со статусом "submitted"
// до тех пор, пока все они не перейдут в "completed" или "failed".
// Задачи без зарегистрированного адаптера (заглушки) пропускаются.
func (o *Orchestrator) pollUntilDone(ctx context.Context, results []models.ExecutionResult) ([]models.ExecutionResult, error) {
	interval := o.PollInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	timeout := o.PollTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Считаем задачи, которые реально можно дождаться.
		pending := 0
		for _, r := range results {
			if r.Status == "submitted" && r.JobID != "" {
				if _, ok := o.registry.Get(r.Provider); ok {
					pending++
				}
			}
		}
		if pending == 0 {
			return results, nil
		}

		select {
		case <-ctx.Done():
			return results, fmt.Errorf("timed out waiting for %d job(s): %w", pending, ctx.Err())
		case <-ticker.C:
			for i, r := range results {
				if r.Status != "submitted" || r.JobID == "" {
					continue
				}
				adapter, ok := o.registry.Get(r.Provider)
				if !ok {
					continue
				}
				updated, err := adapter.Status(ctx, r.JobID)
				if err != nil {
					results[i].Status = "failed"
					results[i].Error = err.Error()
					continue
				}
				shotID := r.ShotID // Status() не знает ShotID — сохраняем
				results[i] = updated
				results[i].ShotID = shotID
			}
		}
	}
}
