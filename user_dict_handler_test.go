package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/shinshin86/vpeak"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	dictPath := filepath.Join(t.TempDir(), "dic.json")
	server := NewServer(serverConfig{userDictPath: dictPath}, log.New(io.Discard, "", 0))
	return server, dictPath
}

func TestGetUserDictReturnsEmptyArray(t *testing.T) {
	server, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/user_dict", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "[]\n" {
		t.Fatalf("expected empty array, got %q", body)
	}
}

func TestUserDictCRUDBySurface(t *testing.T) {
	server, dictPath := newTestServer(t)

	createReq := httptest.NewRequest(http.MethodPost, "/user_dict_word?surface=GitHub&pronunciation=ギットハブ&accent_type=0&priority=5", nil)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/user_dict", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	var words []UserDictWord
	if err := json.Unmarshal(getRec.Body.Bytes(), &words); err != nil {
		t.Fatalf("failed to decode user dict: %v", err)
	}
	if len(words) != 1 || words[0].Pronunciation != "ギットハブ" {
		t.Fatalf("expected pronunciation to be saved, got %+v", words)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/user_dict_word/by-surface/GitHub?surface=GitHub%20Actions&pronunciation=ギットハブアクションズ&accent_type=1&priority=7", nil)
	updateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	loaded, err := vpeak.LoadDictionary(dictPath)
	if err != nil {
		t.Fatalf("LoadDictionary() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Surface != "GitHub Actions" || loaded[0].AccentType != 1 {
		t.Fatalf("expected updated dictionary entry, got %+v", loaded)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/user_dict_word/by-surface/GitHub%20Actions", nil)
	deleteRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	loaded, err = vpeak.LoadDictionary(dictPath)
	if err != nil {
		t.Fatalf("LoadDictionary() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty dictionary, got %+v", loaded)
	}
}

func TestUserDictConflictBySurface(t *testing.T) {
	server, dictPath := newTestServer(t)
	raw := `[
  {"sur":"git","pron":"ギット","pos":"Japanese_Futsuu_meishi","priority":5,"accentType":0,"lang":"ja"},
  {"sur":"git","pron":"ギット","pos":"Japanese_Futsuu_meishi","priority":5,"accentType":0,"lang":"ja"}
]`
	if err := os.WriteFile(dictPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/user_dict_word/by-surface/git", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestImportUserDictOverride(t *testing.T) {
	server, dictPath := newTestServer(t)
	if err := vpeak.SaveDictionary(dictPath, []vpeak.DictEntry{
		{Surface: "GitHub", Pronunciation: "ギットハブ", Pos: "Japanese_Koyuumeishi_ippan", Priority: 5, AccentType: 0, Lang: "ja"},
	}); err != nil {
		t.Fatalf("SaveDictionary() error = %v", err)
	}

	body := []byte(`[
  {
    "sur": "GitHub",
    "pron": "ギットハブアクションズ",
    "pos": "Japanese_Futsuu_meishi",
    "priority": 6,
    "accentType": 1,
    "lang": "ja"
  }
]`)

	req := httptest.NewRequest(http.MethodPost, "/import_user_dict?override=true", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	loaded, err := vpeak.LoadDictionary(dictPath)
	if err != nil {
		t.Fatalf("LoadDictionary() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Pos != "Japanese_Futsuu_meishi" || loaded[0].AccentType != 1 {
		t.Fatalf("expected imported dictionary entry, got %+v", loaded)
	}
}

func TestUserDictValidationReturns422(t *testing.T) {
	server, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/user_dict_word?surface=GitHub&pronunciation=abc&accent_type=0", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}
}
