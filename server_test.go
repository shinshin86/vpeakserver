package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shinshin86/vpeak"
)

func newBaseTestServer() *Server {
	return NewServer(serverConfig{}, log.New(io.Discard, "", 0))
}

func TestValidateEmotionOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "single name", input: "happy", want: "happy"},
		{name: "single weighted", input: "happy=50", want: "happy=50"},
		{name: "multi weighted", input: "happy=40,fun=60", want: "happy=40,fun=60"},
		{name: "trim spaces", input: " happy=40,fun=60 ", want: "happy=40,fun=60"},
		{name: "invalid name", input: "joy=50", wantErr: true},
		{name: "invalid integer", input: "happy=foo", wantErr: true},
		{name: "too high", input: "happy=101", wantErr: true},
		{name: "negative", input: "happy=-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateEmotionOption(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateEmotionOption(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("validateEmotionOption(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAudioQueryEmotionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		emotion    string
		wantStatus int
		wantBody   string
	}{
		{name: "empty", emotion: "", wantStatus: http.StatusOK, wantBody: ""},
		{name: "single name", emotion: "happy", wantStatus: http.StatusOK, wantBody: "happy"},
		{name: "single weighted", emotion: "happy=50", wantStatus: http.StatusOK, wantBody: "happy=50"},
		{name: "multi weighted", emotion: "happy=40,fun=60", wantStatus: http.StatusOK, wantBody: "happy=40,fun=60"},
		{name: "invalid name", emotion: "joy=50", wantStatus: http.StatusBadRequest},
		{name: "invalid integer", emotion: "happy=foo", wantStatus: http.StatusBadRequest},
		{name: "too high", emotion: "happy=101", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newBaseTestServer()
			req := httptest.NewRequest(http.MethodPost, "/audio_query?text=hello&speaker=f1&emotion="+tt.emotion, nil)
			rec := httptest.NewRecorder()

			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}

			if tt.wantStatus != http.StatusOK {
				if !strings.Contains(rec.Body.String(), "Invalid emotion parameter") {
					t.Fatalf("expected invalid emotion error, got %q", rec.Body.String())
				}
				return
			}

			var query AudioQuery
			if err := json.Unmarshal(rec.Body.Bytes(), &query); err != nil {
				t.Fatalf("failed to decode audio query: %v", err)
			}
			if query.Emotion != tt.wantBody {
				t.Fatalf("expected emotion %q, got %q", tt.wantBody, query.Emotion)
			}
		})
	}
}

func TestSynthesisEmotionValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		body            string
		wantStatus      int
		wantEmotion     string
		wantGenerator   bool
		generatorErr    error
		wantErrorSubstr string
	}{
		{
			name:          "empty emotion",
			body:          `{"text":"hello","speaker":"f1","emotion":""}`,
			wantStatus:    http.StatusOK,
			wantEmotion:   "",
			wantGenerator: true,
		},
		{
			name:          "single name",
			body:          `{"text":"hello","speaker":"f1","emotion":"happy"}`,
			wantStatus:    http.StatusOK,
			wantEmotion:   "happy",
			wantGenerator: true,
		},
		{
			name:          "single weighted",
			body:          `{"text":"hello","speaker":"f1","emotion":"happy=50"}`,
			wantStatus:    http.StatusOK,
			wantEmotion:   "happy=50",
			wantGenerator: true,
		},
		{
			name:          "multi weighted",
			body:          `{"text":"hello","speaker":"f1","emotion":"happy=40,fun=60"}`,
			wantStatus:    http.StatusOK,
			wantEmotion:   "happy=40,fun=60",
			wantGenerator: true,
		},
		{
			name:            "invalid name",
			body:            `{"text":"hello","speaker":"f1","emotion":"joy=50"}`,
			wantStatus:      http.StatusBadRequest,
			wantErrorSubstr: "Invalid emotion",
		},
		{
			name:            "invalid integer",
			body:            `{"text":"hello","speaker":"f1","emotion":"happy=foo"}`,
			wantStatus:      http.StatusBadRequest,
			wantErrorSubstr: "Invalid emotion",
		},
		{
			name:            "too high",
			body:            `{"text":"hello","speaker":"f1","emotion":"happy=101"}`,
			wantStatus:      http.StatusBadRequest,
			wantErrorSubstr: "Invalid emotion",
		},
		{
			name:            "generator error",
			body:            `{"text":"hello","speaker":"f1","emotion":"happy=40,fun=60"}`,
			wantStatus:      http.StatusInternalServerError,
			wantEmotion:     "happy=40,fun=60",
			wantGenerator:   true,
			generatorErr:    errors.New("boom"),
			wantErrorSubstr: "Failed to generate speech",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newBaseTestServer()
			called := false
			var capturedText string
			var capturedOpts vpeak.Options

			server.speechGenerator = func(text string, opts vpeak.Options) error {
				called = true
				capturedText = text
				capturedOpts = opts
				if tt.generatorErr != nil {
					return tt.generatorErr
				}

				return os.WriteFile(opts.Output, []byte("RIFF"), 0o644)
			}

			req := httptest.NewRequest(http.MethodPost, "/synthesis", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			server.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tt.wantStatus, rec.Code, rec.Body.String())
			}

			if called != tt.wantGenerator {
				t.Fatalf("speech generator called = %v, want %v", called, tt.wantGenerator)
			}

			if tt.wantGenerator {
				if capturedText != "hello" {
					t.Fatalf("expected text %q, got %q", "hello", capturedText)
				}
				if capturedOpts.Narrator != "f1" {
					t.Fatalf("expected narrator %q, got %q", "f1", capturedOpts.Narrator)
				}
				if capturedOpts.Emotion != tt.wantEmotion {
					t.Fatalf("expected emotion %q, got %q", tt.wantEmotion, capturedOpts.Emotion)
				}
				if capturedOpts.Output == "" {
					t.Fatal("expected output path to be set")
				}
			}

			if tt.wantStatus == http.StatusOK {
				if got := rec.Header().Get("Content-Type"); got != "audio/wav" {
					t.Fatalf("expected content-type audio/wav, got %q", got)
				}
				if rec.Body.Len() == 0 {
					t.Fatal("expected wav response body")
				}
				return
			}

			if tt.wantErrorSubstr != "" && !strings.Contains(rec.Body.String(), tt.wantErrorSubstr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErrorSubstr, rec.Body.String())
			}
		})
	}
}
