package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/shinshin86/vpeak"
)

var allowedOrigin string
var corsPolicyMode string

type AudioQuery struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker"`
	Emotion string `json:"emotion"`
}

type SettingsData struct {
	CorsPolicyMode string
	AllowOrigin    string
}

// Middleware to handle CORS
func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if corsPolicyMode == "all" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if corsPolicyMode == "localapps" {
			if strings.HasPrefix(origin, "app://") || strings.HasPrefix(origin, "http://localhost") || origin == allowedOrigin || containsOrigin(allowedOrigin, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
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

func containsOrigin(allowedOrigins string, origin string) bool {
	origins := strings.Split(allowedOrigins, " ")
	for _, o := range origins {
		if o == origin {
			return true
		}
	}
	return false
}

func main() {
	flag.StringVar(&allowedOrigin, "allowed-origin", "", "Set the allowed CORS origin")
	flag.StringVar(&corsPolicyMode, "cors-policy-mode", "localapps", "Set the CORS policy mode (localapps or all)")
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

	// Add the settings page handler
	http.HandleFunc("/setting", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			settingsHTML := `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>vpeakserver 設定</title>
  <style>
    body {
      font-family: sans-serif;
      margin: 20px;
    }
    h1 {
      font-size: 1.5rem;
      margin-bottom: 1rem;
    }
    .alert {
      background-color: #fff7d5; /* 薄い黄色 */
      padding: 1rem;
      margin-bottom: 1.5rem;
      border: 1px solid #f0e9c6;
    }
    label {
      display: block;
      font-weight: bold;
      margin: 1rem 0 0.5rem;
    }
    select, input[type="text"] {
      width: 300px;
      padding: 0.5rem;
      font-size: 1rem;
      margin-bottom: 0.5rem;
    }
    .description {
      font-size: 0.9rem;
      color: #555;
      margin-bottom: 1rem;
    }
    .success-message {
      background-color: #d4edda;
      color: #155724;
      padding: 1rem;
      margin-bottom: 1.5rem;
      border: 1px solid #c3e6cb;
      display: none;
    }
  </style>
</head>
<body>

  <h1>vpeakserver 設定</h1>

  <div class="alert">
    変更を反映するには音声合成エンジンの再起動が必要です。
  </div>

  <div id="successMessage" class="success-message">
    設定が保存されました。変更を完全に適用するには音声合成エンジンの再起動が必要です。
  </div>

  <form id="settingsForm">
    <label for="corsPolicyMode">CORS Policy Mode</label>
    <select id="corsPolicyMode" name="corsPolicyMode">
      <option value="localapps" {{if eq .CorsPolicyMode "localapps"}}selected{{end}}>localapps</option>
      <option value="all" {{if eq .CorsPolicyMode "all"}}selected{{end}}>all</option>
    </select>
    <div class="description">
      <strong>localapps</strong> はオリジン間リソース共有ポリシーを、
      <code>app://</code> と <code>localhost</code> 関連に限定します。<br>
      その他のオリジンは <strong>Allow Origin</strong> オプションで追加できます。<br>
      <strong>all</strong> はすべてを許可します。危険性を理解した上でご利用ください。
    </div>

    <label for="allowOrigin">Allow Origin</label>
    <input id="allowOrigin" name="allowOrigin" type="text" 
           value="{{.AllowOrigin}}">
    <div class="description">
      許可するオリジンを指定します。スペースで区切ることで複数指定できます。
    </div>
  </form>

  <script>
    document.getElementById('corsPolicyMode').addEventListener('change', saveSettings);
    document.getElementById('allowOrigin').addEventListener('blur', saveSettings);

    function saveSettings() {
      const corsPolicyMode = document.getElementById('corsPolicyMode').value;
      const allowOrigin = document.getElementById('allowOrigin').value;
      
      fetch('/update-settings', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          corsPolicyMode: corsPolicyMode,
          allowOrigin: allowOrigin
        })
      })
      .then(response => {
        if (response.ok) {
          const successMessage = document.getElementById('successMessage');
          successMessage.style.display = 'block';
          setTimeout(() => {
            successMessage.style.display = 'none';
          }, 3000);
        }
      })
      .catch(error => {
        console.error('設定の保存中にエラーが発生しました:', error);
      });
    }
  </script>
</body>
</html>`

			tmpl, err := template.New("settings").Parse(settingsHTML)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
				return
			}

			data := SettingsData{
				CorsPolicyMode: corsPolicyMode,
				AllowOrigin:    allowedOrigin,
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.Execute(w, data); err != nil {
				http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
				return
			}
		}
	})

	// Update settings endpoint
	http.HandleFunc("/update-settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var settings SettingsData
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusBadRequest)
			return
		}

		corsPolicyMode = settings.CorsPolicyMode
		allowedOrigin = settings.AllowOrigin

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success"}`))
	})

	fmt.Println("Server started at http://localhost:20202")
	fmt.Printf("Starting server with allowed origin: %s\n", allowedOrigin)
	fmt.Printf("CORS policy mode: %s\n", corsPolicyMode)
	log.Fatal(http.ListenAndServe(":20202", nil))
}
