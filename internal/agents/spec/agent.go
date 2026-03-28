// Пакет `spec` реализует агента спецификации, который преобразует
// сырой пользовательский промпт в структурированный `Spec`. Если промпт
// неоднозначен, агент возвращает уточняющие вопросы вместо продолжения;
// оркестратор вызывает его повторно, когда ответы уже получены.
package spec

import (
	"context"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

// `Agent` - интерфейс, который вызывает оркестратор.
// Локальное размещение в этом пакете означает, что оркестратор
// зависит от этого интерфейса, а не от общего "мега-интерфейса".
type Agent interface {
	// `Clarify` анализирует промпт и возвращает вопросы,
	// ответы на которые нужны агенту до построения `Spec`.
	// Возвращает пустой срез, если промпт достаточно ясен.
	Clarify(ctx context.Context, prompt models.RawPrompt) ([]models.ClarifyingQuestion, error)

	// `Build` создает структурированный `Spec` из промпта и
	// ответов на ранее заданные уточняющие вопросы.
	Build(ctx context.Context, prompt models.RawPrompt, answers []models.ClarifyingAnswer) (models.Spec, error)
}

// `stubAgent` - стартовая no-op реализация для периода разработки.
// Заменяется реализацией на базе LLM без изменения мест вызова.
type stubAgent struct{}

// `New` возвращает stub-агента. Когда wiring с LLM будет готов,
// замените возвращаемый тип на реальную реализацию.
func New() Agent {
	return &stubAgent{}
}

func (a *stubAgent) Clarify(_ context.Context, _ models.RawPrompt) ([]models.ClarifyingQuestion, error) {
	// Заглушка: считаем, что промпты всегда достаточно ясные.
	// Реальная реализация вызывает LLM и парсит структурированный
	// вывод, чтобы определить неоднозначность.
	return nil, nil
}

func (a *stubAgent) Build(_ context.Context, prompt models.RawPrompt, _ []models.ClarifyingAnswer) (models.Spec, error) {
	// Заглушка: возвращаем минимальный `Spec`, заполненный сырым текстом.
	return models.Spec{
		ID:           "spec-stub-001",
		PromptID:     prompt.ID,
		Narrative:    prompt.Text,
		OutputFormat: "image",
		AspectRatio:  "16:9",
		CreatedAt:    time.Now(),
	}, nil
}
