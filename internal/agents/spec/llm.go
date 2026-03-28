package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

const (
	codexBaseURL = "https://api.openai.com/v1/chat/completions"
	defaultModel = "gpt-5"
)

// llmAgent - реализация Spec Agent на базе Codex/OpenAI API.
// Для структурированного вывода используется JSON Schema response_format.
type llmAgent struct {
	apiKey string
	model  string
	client *http.Client
}

// NewLLM создает Spec Agent на базе Codex/OpenAI API.
func NewLLM(apiKey string) Agent {
	model := strings.TrimSpace(os.Getenv("CODEX_MODEL"))
	if model == "" {
		model = defaultModel
	}
	return &llmAgent{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (a *llmAgent) Clarify(ctx context.Context, prompt models.RawPrompt) ([]models.ClarifyingQuestion, error) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"questions": {
				"type": "array",
				"description": "Список уточняющих вопросов. Пустой массив если промпт достаточно ясен.",
				"items": {
					"type": "object",
					"properties": {
						"id":       {"type": "string", "description": "Уникальный идентификатор вопроса, например q1, q2"},
						"question": {"type": "string"},
						"hint":     {"type": "string", "description": "Варианты ответа через /"}
					},
					"required": ["id", "question", "hint"],
					"additionalProperties": false
				}
			}
		},
		"required": ["questions"],
		"additionalProperties": false
	}`)

	systemText := `Ты аналитик визуальных медиа-запросов. Проверь сырой промпт для AI-генерации изображения или видео.

Определи, каких критичных данных не хватает:
- output_format: image, video или sequence — КРИТИЧНО, без этого невозможно выбрать провайдера
- aspect_ratio: 16:9, 9:16, 1:1 — КРИТИЧНО, без этого нельзя сформировать запрос
- style: кинематографический, аниме, фотореализм, живопись и т.д.
- mood: эмоциональный тон сцены
- environment: место и время действия (только если совсем не указаны)

Задай вопросы ТОЛЬКО по реально отсутствующим полям. Максимум 5 вопросов.
Не спрашивай о том, что можно разумно вывести из контекста.
Если промпт достаточно ясен — верни пустой массив questions.
К каждому вопросу добавь hint с конкретными вариантами ответа.`

	contentJSON, err := a.callJSONSchema(ctx, systemText, prompt.Text, "return_questions", schema)
	if err != nil {
		return nil, fmt.Errorf("spec clarify: %w", err)
	}

	var result struct {
		Questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
			Hint     string `json:"hint"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(contentJSON, &result); err != nil {
		return nil, fmt.Errorf("spec clarify parse: %w", err)
	}

	questions := make([]models.ClarifyingQuestion, len(result.Questions))
	for i, q := range result.Questions {
		questions[i] = models.ClarifyingQuestion{
			ID:       q.ID,
			Question: q.Question,
			Hint:     q.Hint,
		}
	}
	return questions, nil
}

func (a *llmAgent) Build(ctx context.Context, prompt models.RawPrompt, answers []models.ClarifyingAnswer) (models.Spec, error) {
	userText := buildUserText(prompt.Text, answers)

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"characters":       {"type": "array",  "items": {"type": "string"}, "description": "Персонажи или объекты в кадре"},
			"environment":      {"type": "string", "description": "Место и время действия"},
			"mood":             {"type": "string", "description": "Эмоциональный тон сцены"},
			"narrative":        {"type": "string", "description": "Полное описание сцены своими словами"},
			"style":            {"type": "string", "description": "Визуальный стиль"},
			"color_palette":    {"type": "array",  "items": {"type": "string"}, "description": "Цвета или палитра"},
			"lighting":         {"type": "string", "description": "Тип освещения"},
			"references":       {"type": "array",  "items": {"type": "string"}, "description": "Референсы стиля"},
			"camera_angle":     {"type": "string", "description": "Угол камеры"},
			"camera_motion":    {"type": "string", "description": "Движение камеры"},
			"motion_intensity": {"type": "string", "enum": ["subtle", "moderate", "dynamic"]},
			"output_format":    {"type": "string", "enum": ["image", "video", "sequence"], "description": "Формат вывода. По умолчанию image."},
			"aspect_ratio":     {"type": "string", "description": "Соотношение сторон. По умолчанию 16:9."},
			"duration_sec":     {"type": "integer", "description": "Длительность в секундах, только для video"},
			"restrictions":     {"type": "array",  "items": {"type": "string"}, "description": "Что не должно быть в кадре"}
		},
		"required": ["output_format", "aspect_ratio", "narrative"],
		"additionalProperties": false
	}`)

	systemText := `Ты агент построения спецификации для AI-генерации визуальных медиа.
Извлеки все возможные поля из промпта и ответов пользователя.

Правила:
- output_format: "image" по умолчанию если не указано явно
- aspect_ratio: "16:9" по умолчанию если не указано явно
- narrative: напиши полное описание сцены своими словами на основе всех данных
- Заполняй только поля, реально следующие из текста. Не придумывай детали.
- duration_sec заполняй только для output_format="video"`

	contentJSON, err := a.callJSONSchema(ctx, systemText, userText, "build_spec", schema)
	if err != nil {
		return models.Spec{}, fmt.Errorf("spec build: %w", err)
	}

	var raw struct {
		Characters      []string `json:"characters"`
		Environment     string   `json:"environment"`
		Mood            string   `json:"mood"`
		Narrative       string   `json:"narrative"`
		Style           string   `json:"style"`
		ColorPalette    []string `json:"color_palette"`
		Lighting        string   `json:"lighting"`
		References      []string `json:"references"`
		CameraAngle     string   `json:"camera_angle"`
		CameraMotion    string   `json:"camera_motion"`
		MotionIntensity string   `json:"motion_intensity"`
		OutputFormat    string   `json:"output_format"`
		AspectRatio     string   `json:"aspect_ratio"`
		DurationSec     int      `json:"duration_sec"`
		Restrictions    []string `json:"restrictions"`
	}
	if err := json.Unmarshal(contentJSON, &raw); err != nil {
		return models.Spec{}, fmt.Errorf("spec build parse: %w", err)
	}

	return models.Spec{
		ID:              fmt.Sprintf("spec-%d", time.Now().UnixNano()),
		PromptID:        prompt.ID,
		Characters:      raw.Characters,
		Environment:     raw.Environment,
		Mood:            raw.Mood,
		Narrative:       raw.Narrative,
		Style:           raw.Style,
		ColorPalette:    raw.ColorPalette,
		Lighting:        raw.Lighting,
		References:      raw.References,
		CameraAngle:     raw.CameraAngle,
		CameraMotion:    raw.CameraMotion,
		MotionIntensity: raw.MotionIntensity,
		OutputFormat:    raw.OutputFormat,
		AspectRatio:     raw.AspectRatio,
		DurationSec:     raw.DurationSec,
		Restrictions:    raw.Restrictions,
		CreatedAt:       time.Now(),
	}, nil
}

type llmRequest struct {
	Model          string            `json:"model"`
	Messages       []llmMessage      `json:"messages"`
	ResponseFormat llmResponseFormat `json:"response_format"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmResponseFormat struct {
	Type       string        `json:"type"`
	JSONSchema llmJSONSchema `json:"json_schema"`
}

type llmJSONSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type llmResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *llmAgent) callJSONSchema(ctx context.Context, systemText, userText, schemaName string, schema json.RawMessage) ([]byte, error) {
	req := llmRequest{
		Model: a.model,
		Messages: []llmMessage{
			{Role: "system", Content: systemText},
			{Role: "user", Content: userText},
		},
		ResponseFormat: llmResponseFormat{
			Type: "json_schema",
			JSONSchema: llmJSONSchema{
				Name:   schemaName,
				Strict: true,
				Schema: schema,
			},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, codexBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex api %d: %s", httpResp.StatusCode, respBody)
	}

	var resp llmResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("codex api error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("codex api: empty choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("codex api: empty response content")
	}

	var validate any
	if err := json.Unmarshal([]byte(content), &validate); err != nil {
		return nil, fmt.Errorf("codex api: non-json content: %w", err)
	}
	return []byte(content), nil
}

func buildUserText(promptText string, answers []models.ClarifyingAnswer) string {
	if len(answers) == 0 {
		return promptText
	}
	var sb strings.Builder
	sb.WriteString(promptText)
	sb.WriteString("\n\nОтветы на уточняющие вопросы:\n")
	for _, ans := range answers {
		fmt.Fprintf(&sb, "- [%s]: %s\n", ans.QuestionID, ans.Answer)
	}
	return sb.String()
}

