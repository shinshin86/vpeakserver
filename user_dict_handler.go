package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/shinshin86/vpeak"
)

func (s *Server) handleUserDict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	dictPath, err := s.resolveUserDictPath()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve dictionary path: %v", err), http.StatusInternalServerError)
		return
	}

	entries, err := vpeak.LoadDictionary(dictPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load dictionary: %v", err), http.StatusInternalServerError)
		return
	}

	words := make([]UserDictWord, 0, len(entries))
	for _, entry := range entries {
		words = append(words, userDictWordFromEntry(entry))
	}

	writeJSON(w, http.StatusOK, words)
}

func (s *Server) handleCreateUserDictWord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	entry, err := parseUserDictWordRequest(r.URL.Query())
	if err != nil {
		writeValidationError(w, err)
		return
	}

	dictPath, err := s.resolveUserDictPath()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve dictionary path: %v", err), http.StatusInternalServerError)
		return
	}

	if err := vpeak.AddDictionaryWord(dictPath, entry); err != nil {
		writeDictionaryError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, userDictWordFromEntry(entry))
}

func (s *Server) handleUserDictWordBySurface(w http.ResponseWriter, r *http.Request) {
	surface, err := parseSurfacePath(strings.TrimPrefix(r.URL.Path, "/user_dict_word/by-surface/"))
	if err != nil {
		writeValidationError(w, err)
		return
	}

	dictPath, err := s.resolveUserDictPath()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve dictionary path: %v", err), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodPut:
		entry, err := parseUserDictWordRequest(r.URL.Query())
		if err != nil {
			writeValidationError(w, err)
			return
		}

		if err := vpeak.UpdateDictionaryWordBySurface(dictPath, surface, entry); err != nil {
			writeDictionaryError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := vpeak.DeleteDictionaryWordBySurface(dictPath, surface); err != nil {
			writeDictionaryError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Only PUT and DELETE methods are allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleImportUserDict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	overrideRaw := r.URL.Query().Get("override")
	if overrideRaw == "" {
		writeValidationError(w, fmt.Errorf("override is required"))
		return
	}

	override := false
	switch overrideRaw {
	case "true":
		override = true
	case "false":
	default:
		writeValidationError(w, fmt.Errorf("override must be true or false"))
		return
	}

	var entries []vpeak.DictEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		writeValidationError(w, fmt.Errorf("request body must be a JSON array of dictionary entries"))
		return
	}

	dictPath, err := s.resolveUserDictPath()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve dictionary path: %v", err), http.StatusInternalServerError)
		return
	}

	if err := vpeak.ImportDictionary(dictPath, entries, override); err != nil {
		writeDictionaryError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseSurfacePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "/") {
		return "", fmt.Errorf("surface is invalid")
	}

	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		return "", fmt.Errorf("surface is invalid")
	}

	unescaped = strings.TrimSpace(unescaped)
	if unescaped == "" {
		return "", fmt.Errorf("surface is invalid")
	}

	return unescaped, nil
}

func writeDictionaryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vpeak.ErrDictionaryWordInvalid):
		writeValidationError(w, err)
	case errors.Is(err, vpeak.ErrDictionaryWordNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, vpeak.ErrDictionaryWordConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, fmt.Sprintf("dictionary operation failed: %v", err), http.StatusInternalServerError)
	}
}

func writeValidationError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
		"detail": err.Error(),
	})
}
