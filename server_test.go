package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newBaseTestServer() *Server {
	return NewServer(serverConfig{}, log.New(io.Discard, "", 0))
}

func TestAudioQueryAcceptsWeightedEmotion(t *testing.T) {
	server := newBaseTestServer()
	req := httptest.NewRequest(http.MethodPost, "/audio_query?text=hello&speaker=f1&emotion=happy=40,fun=60", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var query AudioQuery
	if err := json.Unmarshal(rec.Body.Bytes(), &query); err != nil {
		t.Fatalf("failed to decode audio query: %v", err)
	}

	if query.Emotion != "happy=40,fun=60" {
		t.Fatalf("expected weighted emotion to be preserved, got %q", query.Emotion)
	}
}

func TestAudioQueryRejectsInvalidEmotion(t *testing.T) {
	server := newBaseTestServer()
	req := httptest.NewRequest(http.MethodPost, "/audio_query?text=hello&speaker=f1&emotion=joy=50", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Invalid emotion parameter") {
		t.Fatalf("expected invalid emotion error, got %q", rec.Body.String())
	}
}

func TestSynthesisRejectsInvalidEmotion(t *testing.T) {
	server := newBaseTestServer()
	body := []byte(`{"text":"hello","speaker":"f1","emotion":"happy=101"}`)
	req := httptest.NewRequest(http.MethodPost, "/synthesis", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Invalid emotion") {
		t.Fatalf("expected invalid emotion error, got %q", rec.Body.String())
	}
}
