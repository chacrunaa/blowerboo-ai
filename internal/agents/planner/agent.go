// Пакет planner реализует агента планирования, который преобразует
// структурированный `Spec` в упорядоченный список шотов (`Plan`).
package planner

import (
	"context"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

// Agent - интерфейс, который вызывает оркестратор.
type Agent interface {
	// Plan принимает финализированный `Spec` и возвращает `Plan`
	// с упорядоченным списком `Shot`.
	Plan(ctx context.Context, spec models.Spec) (models.Plan, error)
}

type stubAgent struct{}

// New возвращает stub-агента.
func New() Agent {
	return &stubAgent{}
}

func (a *stubAgent) Plan(_ context.Context, spec models.Spec) (models.Plan, error) {
	// Заглушка: формируем одношотовый план из spec.
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
