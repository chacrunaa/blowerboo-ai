// Пакет `execution` реализует агента выполнения, который
// преобразует `Plan` в готовые для провайдера `ExecutionPayload` и
// при необходимости отправляет их через реестр провайдеров.
package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
	"github.com/blowerboo/blowerboo/internal/providers"
)

// `Agent` - интерфейс, который вызывает оркестратор.
type Agent interface {
	// `Format` преобразует каждый `Shot` из плана в
	// `ExecutionPayload`, нацеленный на конкретного провайдера.
	// Если для шота не указан предпочтительный провайдер,
	// агент выбирает первый зарегистрированный совместимый адаптер.
	Format(ctx context.Context, plan models.Plan, spec models.Spec, registry *providers.Registry) ([]models.ExecutionPayload, error)

	// `Submit` отправляет все payload-ы и собирает результаты.
	// Это отдельный шаг, чтобы вызывающая сторона могла проверить payload-ы
	// до выполнения API-вызовов.
	Submit(ctx context.Context, payloads []models.ExecutionPayload, registry *providers.Registry) ([]models.ExecutionResult, error)
}

type stubAgent struct{}

// `New` возвращает stub-агента.
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
			// Резерв заглушки: помечаем как отправленное без реального вызова.
			results = append(results, models.ExecutionResult{
				ShotID:    p.ShotID,
				Provider:  p.Provider,
				Status:    "submitted",
				JobID:     fmt.Sprintf("stub-job-%s", p.ShotID),
				CreatedAt: time.Now(),
			})
			continue
		}
		// Путь реального адаптера: используется после подключения реальных провайдеров.
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
