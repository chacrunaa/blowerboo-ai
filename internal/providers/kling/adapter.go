// Package kling implements a providers.Adapter for Kling AI (klingai.com).
// It supports both image and short-video generation via the Kling REST API.
//
// Authentication:
//   - JWT mode (recommended): set KLING_ACCESS_KEY + KLING_SECRET_KEY
//   - Bearer mode (fallback):  set KLING_API_KEY
//
// If both are set, JWT mode takes precedence.
package kling

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
)

// ---- constants ---------------------------------------------------------------

const (
	providerName   = "kling"
	defaultBaseURL = "https://api.klingai.com"
	maxDurationSec = 10

	jobPrefixVideo = "video:"
	jobPrefixImage = "image:"

	klingStatusSubmitted  = "submitted"
	klingStatusProcessing = "processing"
	klingStatusSucceed    = "succeed"
	klingStatusFailed     = "failed"

	defaultModel = "kling-v1"
	defaultMode  = "std"
	defaultCfg   = 0.5
)

// ---- auth mode ---------------------------------------------------------------

type authMode int

const (
	authModeBearerKey authMode = iota // KLING_API_KEY used as Bearer token directly
	authModeJWT                       // KLING_ACCESS_KEY + KLING_SECRET_KEY → HS256 JWT
)

// ---- config ------------------------------------------------------------------

// Config holds Kling credentials. Pass to New; or use NewFromEnv to read from
// environment variables.
type Config struct {
	// APIKey is used as a raw Bearer token (authModeBearerKey).
	// Env: KLING_API_KEY
	APIKey string

	// AccessKey and SecretKey are used to generate a short-lived HS256 JWT
	// (authModeJWT). JWT mode takes precedence when both are set.
	// Env: KLING_ACCESS_KEY, KLING_SECRET_KEY
	AccessKey string
	SecretKey string

	// BaseURL overrides the default Kling API endpoint.
	// Env: KLING_BASE_URL (optional)
	BaseURL string
}

// ---- adapter -----------------------------------------------------------------

// Adapter implements providers.Adapter for Kling AI.
type Adapter struct {
	httpClient *http.Client
	// BaseURL is exported so tests can point the adapter at an httptest.Server.
	BaseURL   string
	mode      authMode
	apiKey    string
	accessKey string
	secretKey string
}

// New creates an Adapter from the provided Config. Returns an error if no
// usable credential set is present.
func New(cfg Config) (*Adapter, error) {
	a := &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    defaultBaseURL,
	}
	if cfg.BaseURL != "" {
		a.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	}

	switch {
	case cfg.AccessKey != "" && cfg.SecretKey != "":
		a.mode = authModeJWT
		a.accessKey = cfg.AccessKey
		a.secretKey = cfg.SecretKey
	case cfg.APIKey != "":
		a.mode = authModeBearerKey
		a.apiKey = cfg.APIKey
	default:
		return nil, fmt.Errorf("kling: no credentials — set KLING_API_KEY or KLING_ACCESS_KEY+KLING_SECRET_KEY")
	}

	return a, nil
}

// NewFromEnv reads credentials from environment variables and returns an
// Adapter. Returns an error if no credentials are found.
func NewFromEnv() (*Adapter, error) {
	return New(Config{
		APIKey:    os.Getenv("KLING_API_KEY"),
		AccessKey: os.Getenv("KLING_ACCESS_KEY"),
		SecretKey: os.Getenv("KLING_SECRET_KEY"),
		BaseURL:   os.Getenv("KLING_BASE_URL"),
	})
}

// ---- providers.Adapter interface ---------------------------------------------

// Name returns the canonical provider identifier.
func (a *Adapter) Name() string { return providerName }

// Supports reports whether this adapter can handle the given payload.
func (a *Adapter) Supports(payload models.ExecutionPayload) bool {
	return payload.Provider == providerName &&
		payload.Prompt != "" &&
		payload.DurationSec <= maxDurationSec
}

// Submit sends a generation task to Kling and returns an ExecutionResult with
// Status="submitted" and a JobID encoding the media type and Kling task ID.
func (a *Adapter) Submit(ctx context.Context, payload models.ExecutionPayload) (models.ExecutionResult, error) {
	isVideo := payload.DurationSec > 0

	var (
		path    string
		reqBody []byte
		err     error
	)

	if isVideo {
		path = "/v1/videos/text2video"
		reqBody, err = a.buildVideoRequest(payload)
	} else {
		path = "/v1/images/generations"
		reqBody, err = a.buildImageRequest(payload)
	}
	if err != nil {
		return models.ExecutionResult{}, fmt.Errorf("kling: marshal request: %w", err)
	}

	resp, err := a.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return models.ExecutionResult{}, err
	}

	if resp.Code != 0 {
		return models.ExecutionResult{
			ShotID:    payload.ShotID,
			Provider:  providerName,
			Status:    "failed",
			Error:     fmt.Sprintf("kling API error %d: %s", resp.Code, resp.Message),
			CreatedAt: time.Now(),
		}, nil
	}

	prefix := jobPrefixImage
	if isVideo {
		prefix = jobPrefixVideo
	}

	return models.ExecutionResult{
		ShotID:    payload.ShotID,
		Provider:  providerName,
		JobID:     prefix + resp.Data.TaskID,
		Status:    "submitted",
		CreatedAt: time.Now(),
	}, nil
}

// Status fetches the current state of a previously submitted task.
// jobID must be in the format returned by Submit ("video:<id>" or "image:<id>").
func (a *Adapter) Status(ctx context.Context, jobID string) (models.ExecutionResult, error) {
	path, err := jobIDToPath(jobID)
	if err != nil {
		return models.ExecutionResult{}, err
	}

	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return models.ExecutionResult{}, err
	}

	if resp.Code != 0 {
		return models.ExecutionResult{
			Provider:  providerName,
			JobID:     jobID,
			Status:    "failed",
			Error:     fmt.Sprintf("kling API error %d: %s", resp.Code, resp.Message),
			CreatedAt: time.Now(),
		}, nil
	}

	status := mapStatus(resp.Data.TaskStatus)

	var outputURL string
	if status == "completed" {
		outputURL = extractOutputURL(jobID, resp.Data.TaskResult)
	}

	return models.ExecutionResult{
		Provider:  providerName,
		JobID:     jobID,
		Status:    status,
		OutputURL: outputURL,
		CreatedAt: time.Now(),
	}, nil
}

// ---- Kling API types (unexported) --------------------------------------------

type videoRequest struct {
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	AspectRatio    string  `json:"aspect_ratio,omitempty"`
	Duration       string  `json:"duration"`
	CfgScale       float64 `json:"cfg_scale"`
	Mode           string  `json:"mode"`
}

type imageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	N              int    `json:"n"`
}

type taskResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    taskData `json:"data"`
}

type taskData struct {
	TaskID     string     `json:"task_id"`
	TaskStatus string     `json:"task_status"`
	TaskResult taskResult `json:"task_result"`
}

type taskResult struct {
	Videos []videoItem `json:"videos"`
	Images []imageItem `json:"images"`
}

type videoItem struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Duration string `json:"duration"`
}

type imageItem struct {
	URL string `json:"url"`
}

// ---- request builders --------------------------------------------------------

func (a *Adapter) buildVideoRequest(p models.ExecutionPayload) ([]byte, error) {
	mode := defaultMode
	if v, ok := p.ProviderParams["mode"].(string); ok && v != "" {
		mode = v
	}
	cfg := float64(defaultCfg)
	if v, ok := p.ProviderParams["cfg_scale"].(float64); ok {
		cfg = v
	}
	return json.Marshal(videoRequest{
		Model:          defaultModel,
		Prompt:         p.Prompt,
		NegativePrompt: p.NegativePrompt,
		AspectRatio:    p.AspectRatio,
		Duration:       fmt.Sprintf("%d", p.DurationSec),
		CfgScale:       cfg,
		Mode:           mode,
	})
}

func (a *Adapter) buildImageRequest(p models.ExecutionPayload) ([]byte, error) {
	return json.Marshal(imageRequest{
		Model:          defaultModel,
		Prompt:         p.Prompt,
		NegativePrompt: p.NegativePrompt,
		AspectRatio:    p.AspectRatio,
		N:              1,
	})
}

// ---- HTTP helpers ------------------------------------------------------------

func (a *Adapter) doRequest(ctx context.Context, method, path string, body []byte) (*taskResponse, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("kling: build request: %w", err)
	}

	token, err := a.bearerToken()
	if err != nil {
		return nil, fmt.Errorf("kling: auth token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kling: http: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 500 {
		return nil, fmt.Errorf("kling: server error %d", httpResp.StatusCode)
	}

	var result taskResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("kling: decode response: %w", err)
	}

	return &result, nil
}

// ---- auth helpers ------------------------------------------------------------

func (a *Adapter) bearerToken() (string, error) {
	if a.mode == authModeJWT {
		return a.generateJWT()
	}
	return a.apiKey, nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Iss string `json:"iss"`
	Exp int64  `json:"exp"`
	Nbf int64  `json:"nbf"`
}

// generateJWT creates a short-lived HS256 JWT using stdlib crypto only.
func (a *Adapter) generateJWT() (string, error) {
	headerJSON, err := json.Marshal(jwtHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	now := time.Now()
	claimsJSON, err := json.Marshal(jwtClaims{
		Iss: a.accessKey,
		Exp: now.Add(30 * time.Minute).Unix(),
		Nbf: now.Add(-5 * time.Second).Unix(),
	})
	if err != nil {
		return "", err
	}

	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	c := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := h + "." + c

	mac := hmac.New(sha256.New, []byte(a.secretKey))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}

// ---- misc helpers ------------------------------------------------------------

// jobIDToPath converts a prefixed job ID to the Kling API path for status polling.
func jobIDToPath(jobID string) (string, error) {
	if taskID, ok := strings.CutPrefix(jobID, jobPrefixVideo); ok {
		return "/v1/videos/text2video/" + taskID, nil
	}
	if taskID, ok := strings.CutPrefix(jobID, jobPrefixImage); ok {
		return "/v1/images/generations/" + taskID, nil
	}
	return "", fmt.Errorf("kling: malformed job ID %q (expected prefix %q or %q)", jobID, jobPrefixVideo, jobPrefixImage)
}

// mapStatus converts a Kling task_status string to the internal model status.
// Intermediate values (submitted, processing) and any unknown values map to
// "submitted" so the caller keeps polling.
func mapStatus(klingStatus string) string {
	switch klingStatus {
	case klingStatusSucceed:
		return "completed"
	case klingStatusFailed:
		return "failed"
	case klingStatusSubmitted, klingStatusProcessing:
		return "submitted"
	default:
		return "submitted"
	}
}

// extractOutputURL returns the first available media URL from a task result.
func extractOutputURL(jobID string, result taskResult) string {
	if strings.HasPrefix(jobID, jobPrefixVideo) {
		if len(result.Videos) > 0 {
			return result.Videos[0].URL
		}
		return ""
	}
	if len(result.Images) > 0 {
		return result.Images[0].URL
	}
	return ""
}
