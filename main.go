package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/shinshin86/vpeak"
)

var allowedOrigin string

type AudioQuery struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker"`
	Emotion string `json:"emotion"`
}

// Middleware to handle CORS
func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin || allowedOrigin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

func main() {
	flag.StringVar(&allowedOrigin, "allowed-origin", "http://localhost:3000", "Set the allowed CORS origin")
	flag.Parse()

	http.HandleFunc("/audio_query", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		text := r.URL.Query().Get("text")
		speaker := r.URL.Query().Get("speaker")

		if text == "" || speaker == "" {
			http.Error(w, "Missing required parameters: text and speaker", http.StatusBadRequest)
			return
		}

		audioQuery := AudioQuery{
			Text:    text,
			Speaker: speaker,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(audioQuery); err != nil {
			http.Error(w, fmt.Sprintf("Failed to encode audio query: %v", err), http.StatusInternalServerError)
			return
		}
	}))

	http.HandleFunc("/synthesis", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var query AudioQuery
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusBadRequest)
			return
		}

		validEmotions := map[string]bool{
			"happy": true,
			"fun":   true,
			"angry": true,
			"sad":   true,
		}
		if !validEmotions[query.Emotion] {
			query.Emotion = ""
		}

		outputFileName := fmt.Sprintf("audio-%s.wav", uuid.New().String())

		opts := vpeak.Options{
			Narrator: query.Speaker,
			Emotion:  query.Emotion,
			Output:   outputFileName,
			Silent:   true,
		}

		if err := vpeak.GenerateSpeech(query.Text, opts); err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate speech: %v", err), http.StatusInternalServerError)
			return
		}

		defer os.Remove(outputFileName)

		w.Header().Set("Content-Type", "audio/wav")
		http.ServeFile(w, r, outputFileName)
	}))

	fmt.Println("Server started at http://localhost:50021")
	fmt.Printf("Starting server with allowed origin: %s\n", allowedOrigin)
	log.Fatal(http.ListenAndServe(":50021", nil))
}
