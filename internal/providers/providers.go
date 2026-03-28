package providers

import (
	"context"

	"github.com/blowerboo/blowerboo/internal/models"
)

// `Adapter` - минимальный интерфейс, который должен
// реализовать каждый медиа-провайдер. Каждый конкретный адаптер (Kling, Runway, Midjourney
// и т.д.) находится в отдельном файле этого пакета.
type Adapter interface {
	// `Name` возвращает канонический идентификатор провайдера,
	// например: "kling", "runway", "midjourney".
	Name() string

	// `Supports` сообщает, может ли адаптер обработать
	// формат и параметры выходного payload-а.
	Supports(payload models.ExecutionPayload) bool

	// `Submit` отправляет payload провайдеру и возвращает
	// `ExecutionResult`. Для асинхронных провайдеров
	// статус результата - "submitted" с `JobID`; вызывающая сторона
	// опрашивает состояние отдельно через `Status`.
	Submit(ctx context.Context, payload models.ExecutionPayload) (models.ExecutionResult, error)

	// `Status` получает текущее состояние ранее
	// отправленной асинхронной задачи. Возвращает тот же тип
	// `ExecutionResult` с обновленным статусом и `OutputURL`, когда результат готов.
	Status(ctx context.Context, jobID string) (models.ExecutionResult, error)
}

// `Registry` - это простой in-process `map` зарегистрированных адаптеров.
// Без рефлексии, без магии, только именованная `map`.
type Registry struct {
	adapters map[string]Adapter
}

// `NewRegistry` возвращает пустой реестр.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]Adapter)}
}

// `Register` добавляет адаптер по его `Name()`. Вызывает `panic`
// при дублирующей регистрации, чтобы поймать ошибки wiring-а на старте.
func (r *Registry) Register(a Adapter) {
	name := a.Name()
	if _, exists := r.adapters[name]; exists {
		panic("providers: duplicate adapter registered: " + name)
	}
	r.adapters[name] = a
}

// `Get` возвращает адаптер по имени. Возвращает `nil, false`,
// если имя не зарегистрировано.
func (r *Registry) Get(name string) (Adapter, bool) {
	a, ok := r.adapters[name]
	return a, ok
}

// `All` возвращает все зарегистрированные адаптеры, что полезно
// для проверки возможностей на этапе планирования выполнения.
func (r *Registry) All() []Adapter {
	out := make([]Adapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		out = append(out, a)
	}
	return out
}
