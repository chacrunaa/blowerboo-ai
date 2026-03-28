package models

import "time"

// ============================================================
// Вход
// ============================================================

// `RawPrompt` - это необработанный пользовательский ввод, который запускает пайплайн.
type RawPrompt struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Выход агента спецификации
// ============================================================

// `ClarifyingQuestion` возвращается агентом спецификации, когда
// промпт неоднозначен. Оркестратор отдает эти вопросы
// вызывающей стороне перед продолжением.
type ClarifyingQuestion struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	// Необязательная подсказка, которая показывается пользователю рядом с вопросом.
	Hint string `json:"hint,omitempty"`
}

// `ClarifyingAnswer` связывает ответ пользователя
// с идентификатором исходного вопроса.
type ClarifyingAnswer struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

// `Spec` - структурированный и валидированный результат работы агента спецификации.
// Все поля - необязательные срезы/строки, чтобы агент
// заполнял только то, что есть в промпте.
type Spec struct {
	ID       string `json:"id"`
	PromptID string `json:"prompt_id"`

	// Нарратив / контент
	Characters  []string `json:"characters,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Mood        string   `json:"mood,omitempty"`
	Narrative   string   `json:"narrative,omitempty"`

	// Визуальное направление
	Style        string   `json:"style,omitempty"`
	ColorPalette []string `json:"color_palette,omitempty"`
	Lighting     string   `json:"lighting,omitempty"`
	References   []string `json:"references,omitempty"` // URL-адреса или описания

	// Камера и движение
	CameraAngle     string `json:"camera_angle,omitempty"`
	CameraMotion    string `json:"camera_motion,omitempty"`
	MotionIntensity string `json:"motion_intensity,omitempty"` // например: "subtle", "dynamic"

	// Выходные параметры
	OutputFormat string `json:"output_format"`           // "image" | "video" | "sequence"
	AspectRatio  string `json:"aspect_ratio,omitempty"`  // например: "16:9", "9:16", "1:1"
	DurationSec  int    `json:"duration_sec,omitempty"`  // для видео

	// Ограничения / негативные промпты
	Restrictions []string `json:"restrictions,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Выход агента планирования
// ============================================================

// `Shot` представляет одну атомарную единицу генерации медиа.
// План состоит из одного или нескольких шотов.
type Shot struct {
	ID          string `json:"id"`
	Order       int    `json:"order"`
	Description string `json:"description"`

	// Каждый шот наследует или переопределяет параметры родительского `Spec`.
	Style        string   `json:"style,omitempty"`
	CameraAngle  string   `json:"camera_angle,omitempty"`
	CameraMotion string   `json:"camera_motion,omitempty"`
	DurationSec  int      `json:"duration_sec,omitempty"`
	Tags         []string `json:"tags,omitempty"`

	// Какие провайдеры предпочтительны для этого шота.
	// Пусто означает "любой совместимый провайдер".
	PreferredProviders []string `json:"preferred_providers,omitempty"`
}

// `Plan` - структурированный план выполнения, который формирует
// агент планирования. Он содержит упорядоченный список шотов и
// общие заметки между шотами.
type Plan struct {
	ID     string `json:"id"`
	SpecID string `json:"spec_id"`

	Shots []Shot `json:"shots"`
	Notes string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Выход агента выполнения
// ============================================================

// `ExecutionPayload` - это независимая от провайдера обертка,
// которую агент выполнения создает для каждого шота. Адаптер провайдера
// переводит ее в нативный API-запрос провайдера.
type ExecutionPayload struct {
	ShotID   string `json:"shot_id"`
	Provider string `json:"provider"` // например: "kling", "runway", "midjourney"

	// Готовый текст промпта для провайдера.
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// Параметры, независимые от провайдера
	AspectRatio string `json:"aspect_ratio,omitempty"`
	DurationSec int    `json:"duration_sec,omitempty"`
	Style       string `json:"style,omitempty"`

	// Запасной путь: специфичные для провайдера переопределения,
	// которые не укладываются в общие поля выше. Адаптер читает их из этой `map`.
	ProviderParams map[string]any `json:"provider_params,omitempty"`
}

// `ExecutionResult` содержит результат, который вернулся от провайдера
// после отправки payload-а.
type ExecutionResult struct {
	ShotID   string `json:"shot_id"`
	Provider string `json:"provider"`

	// `JobID` - ссылка на асинхронную задачу, возвращенная провайдером.
	// Пусто, если результат синхронный.
	JobID  string `json:"job_id,omitempty"`
	Status string `json:"status"` // "submitted" | "completed" | "failed"

	// `OutputURL` заполняется, когда ассет готов.
	OutputURL string    `json:"output_url,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// Обертка пайплайна
// ============================================================

// `Project` - это верхнеуровневый контейнер, который отслеживает
// один сквозной прогон пайплайна. Удобен для сохранения состояния.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`

	Prompt  RawPrompt        `json:"prompt"`
	Spec    *Spec            `json:"spec,omitempty"`
	Plan    *Plan            `json:"plan,omitempty"`
	Results []ExecutionResult `json:"results,omitempty"`

	// Вопросы, заданные на этапе спецификации, и ответы на них.
	Questions []ClarifyingQuestion `json:"questions,omitempty"`
	Answers   []ClarifyingAnswer   `json:"answers,omitempty"`

	Status    string    `json:"status"` // "speccing" | "planning" | "executing" | "done" | "failed"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
