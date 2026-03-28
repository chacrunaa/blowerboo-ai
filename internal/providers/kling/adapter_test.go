package kling_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blowerboo/blowerboo/internal/models"
	"github.com/blowerboo/blowerboo/internal/providers/kling"
)

// ---- helpers -----------------------------------------------------------------

// newAdapter creates a test Adapter pointed at the given server URL.
func newAdapter(t *testing.T, serverURL string) *kling.Adapter {
	t.Helper()
	a, err := kling.New(kling.Config{APIKey: "test-key", BaseURL: serverURL})
	if err != nil {
		t.Fatalf("kling.New: %v", err)
	}
	return a
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// submitResponse is the Kling task submit response shape.
type submitResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"data"`
}

// statusResponse is the Kling status response shape.
type statusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		TaskResult struct {
			Videos []struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"videos"`
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"task_result"`
	} `json:"data"`
}

// ---- TestSupports ------------------------------------------------------------

func TestSupports(t *testing.T) {
	a, err := kling.New(kling.Config{APIKey: "x"})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		payload models.ExecutionPayload
		want    bool
	}{
		{
			name:    "valid video",
			payload: models.ExecutionPayload{Provider: "kling", Prompt: "a fox", DurationSec: 5},
			want:    true,
		},
		{
			name:    "valid image (duration 0)",
			payload: models.ExecutionPayload{Provider: "kling", Prompt: "a fox", DurationSec: 0},
			want:    true,
		},
		{
			name:    "duration at max limit",
			payload: models.ExecutionPayload{Provider: "kling", Prompt: "a fox", DurationSec: 10},
			want:    true,
		},
		{
			name:    "duration exceeds max",
			payload: models.ExecutionPayload{Provider: "kling", Prompt: "a fox", DurationSec: 11},
			want:    false,
		},
		{
			name:    "wrong provider",
			payload: models.ExecutionPayload{Provider: "runway", Prompt: "a fox", DurationSec: 5},
			want:    false,
		},
		{
			name:    "empty prompt",
			payload: models.ExecutionPayload{Provider: "kling", Prompt: "", DurationSec: 5},
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := a.Supports(tc.payload); got != tc.want {
				t.Errorf("Supports() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---- TestName ----------------------------------------------------------------

func TestName(t *testing.T) {
	a, _ := kling.New(kling.Config{APIKey: "x"})
	if got := a.Name(); got != "kling" {
		t.Errorf("Name() = %q, want %q", got, "kling")
	}
}

// ---- TestSubmit_VideoSuccess -------------------------------------------------

func TestSubmit_VideoSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if r.URL.Path != "/v1/videos/text2video" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		// Verify duration is sent as string in request body.
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if dur, ok := body["duration"].(string); !ok || dur != "5" {
			t.Errorf("duration want string %q, got %v", "5", body["duration"])
		}

		var resp submitResponse
		resp.Code = 0
		resp.Message = "success"
		resp.Data.TaskID = "vid-123"
		resp.Data.TaskStatus = "submitted"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Submit(context.Background(), models.ExecutionPayload{
		ShotID:      "shot-1",
		Provider:    "kling",
		Prompt:      "a lone astronaut",
		DurationSec: 5,
	})

	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if result.Status != "submitted" {
		t.Errorf("Status = %q, want %q", result.Status, "submitted")
	}
	if result.JobID != "video:vid-123" {
		t.Errorf("JobID = %q, want %q", result.JobID, "video:vid-123")
	}
	if result.ShotID != "shot-1" {
		t.Errorf("ShotID = %q, want %q", result.ShotID, "shot-1")
	}
	if result.Error != "" {
		t.Errorf("unexpected error field: %q", result.Error)
	}
}

// ---- TestSubmit_ImageSuccess -------------------------------------------------

func TestSubmit_ImageSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		var resp submitResponse
		resp.Code = 0
		resp.Data.TaskID = "img-456"
		resp.Data.TaskStatus = "submitted"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Submit(context.Background(), models.ExecutionPayload{
		ShotID:      "shot-2",
		Provider:    "kling",
		Prompt:      "a misty forest",
		DurationSec: 0, // image
	})

	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if result.JobID != "image:img-456" {
		t.Errorf("JobID = %q, want %q", result.JobID, "image:img-456")
	}
	if result.Status != "submitted" {
		t.Errorf("Status = %q, want %q", result.Status, "submitted")
	}
}

// ---- TestSubmit_APIError -----------------------------------------------------

func TestSubmit_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp submitResponse
		resp.Code = 1400
		resp.Message = "invalid prompt"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Submit(context.Background(), models.ExecutionPayload{
		Provider:    "kling",
		Prompt:      "bad",
		DurationSec: 5,
	})

	if err != nil {
		t.Fatalf("Submit should not return Go error for API errors, got: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("Status = %q, want %q", result.Status, "failed")
	}
	if !strings.Contains(result.Error, "invalid prompt") {
		t.Errorf("Error field %q should contain %q", result.Error, "invalid prompt")
	}
}

// ---- TestSubmit_HTTP5xx ------------------------------------------------------

func TestSubmit_HTTP5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	_, err := a.Submit(context.Background(), models.ExecutionPayload{
		Provider:    "kling",
		Prompt:      "test",
		DurationSec: 5,
	})

	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q should mention status 500", err.Error())
	}
}

// ---- TestStatus_Completed_Video ----------------------------------------------

func TestStatus_Completed_Video(t *testing.T) {
	const taskID = "vid-789"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/videos/text2video/"+taskID {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		var resp statusResponse
		resp.Code = 0
		resp.Data.TaskID = taskID
		resp.Data.TaskStatus = "succeed"
		resp.Data.TaskResult.Videos = []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		}{{ID: "v1", URL: "https://cdn.example.com/v.mp4"}}
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Status(context.Background(), "video:"+taskID)

	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("Status = %q, want %q", result.Status, "completed")
	}
	if result.OutputURL != "https://cdn.example.com/v.mp4" {
		t.Errorf("OutputURL = %q, want %q", result.OutputURL, "https://cdn.example.com/v.mp4")
	}
}

// ---- TestStatus_Completed_Image ---------------------------------------------

func TestStatus_Completed_Image(t *testing.T) {
	const taskID = "img-999"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations/"+taskID {
			t.Errorf("unexpected path %s", r.URL.Path)
		}

		var resp statusResponse
		resp.Code = 0
		resp.Data.TaskStatus = "succeed"
		resp.Data.TaskResult.Images = []struct {
			URL string `json:"url"`
		}{{URL: "https://cdn.example.com/img.png"}}
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Status(context.Background(), "image:"+taskID)

	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("Status = %q, want %q", result.Status, "completed")
	}
	if result.OutputURL != "https://cdn.example.com/img.png" {
		t.Errorf("OutputURL = %q, want %q", result.OutputURL, "https://cdn.example.com/img.png")
	}
}

// ---- TestStatus_Processing --------------------------------------------------

func TestStatus_Processing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp statusResponse
		resp.Code = 0
		resp.Data.TaskStatus = "processing"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Status(context.Background(), "video:vid-proc")

	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if result.Status != "submitted" {
		t.Errorf("Status = %q, want %q (processing maps to submitted)", result.Status, "submitted")
	}
	if result.OutputURL != "" {
		t.Errorf("OutputURL should be empty while processing, got %q", result.OutputURL)
	}
}

// ---- TestStatus_Failed ------------------------------------------------------

func TestStatus_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp statusResponse
		resp.Code = 0
		resp.Data.TaskStatus = "failed"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)
	result, err := a.Status(context.Background(), "video:vid-fail")

	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("Status = %q, want %q", result.Status, "failed")
	}
}

// ---- TestStatus_MalformedJobID ----------------------------------------------

func TestStatus_MalformedJobID(t *testing.T) {
	a, _ := kling.New(kling.Config{APIKey: "x"})
	_, err := a.Status(context.Background(), "raw-task-id-no-prefix")

	if err == nil {
		t.Fatal("expected error for malformed job ID, got nil")
	}
	if !strings.Contains(err.Error(), "malformed job ID") {
		t.Errorf("error %q should mention malformed job ID", err.Error())
	}
}

// ---- TestStatus_ContextTimeout ----------------------------------------------

func TestStatus_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // much longer than the test timeout
		writeJSON(w, http.StatusOK, statusResponse{})
	}))
	defer srv.Close()

	a := newAdapter(t, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := a.Status(ctx, "video:vid-timeout")

	if err == nil {
		t.Fatal("expected error due to context timeout, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("error %q should mention context deadline/cancellation", err.Error())
	}
}

// ---- TestJWT_AuthHeaderFormat -----------------------------------------------

// TestJWT_AuthHeaderFormat verifies that JWT mode produces a valid three-part
// dot-separated Bearer token in the Authorization header.
func TestJWT_AuthHeaderFormat(t *testing.T) {
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		var resp submitResponse
		resp.Code = 0
		resp.Data.TaskID = "jwt-test"
		resp.Data.TaskStatus = "submitted"
		writeJSON(w, http.StatusOK, resp)
	}))
	defer srv.Close()

	a, err := kling.New(kling.Config{
		AccessKey: "my-access-key",
		SecretKey: "my-secret-key",
		BaseURL:   srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	a.Submit(context.Background(), models.ExecutionPayload{
		Provider:    "kling",
		Prompt:      "test",
		DurationSec: 5,
	})

	if !strings.HasPrefix(gotAuthHeader, "Bearer ") {
		t.Fatalf("Authorization header %q should start with 'Bearer '", gotAuthHeader)
	}

	token := strings.TrimPrefix(gotAuthHeader, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWT token should have 3 parts, got %d: %q", len(parts), token)
	}
}
