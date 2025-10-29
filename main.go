package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shinshin86/vpeak"
)

const (
	speedMin = 50
	speedMax = 200
	pitchMin = -300
	pitchMax = 300
)

var allowedOrigin string
var corsPolicyMode string

type AudioQuery struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker"`
	Emotion string `json:"emotion"`
	Speed   *int   `json:"speed,omitempty"`
	Pitch   *int   `json:"pitch,omitempty"`
}

type SettingsData struct {
	CorsPolicyMode string
	AllowOrigin    string
	Lang           string
}

var validEmotions = map[string]bool{
	"happy": true,
	"fun":   true,
	"angry": true,
	"sad":   true,
}

func parseOptionalIntParam(raw string, min, max int) (*int, error) {
	if raw == "" {
		return nil, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to integer: %w", err)
	}

	if value < min || value > max {
		return nil, fmt.Errorf("value must be between %d and %d", min, max)
	}

	return &value, nil
}

func validateOptionalRange(value *int, min, max int) error {
	if value == nil {
		return nil
	}

	val := *value
	if val < min || val > max {
		return fmt.Errorf("value must be between %d and %d", min, max)
	}
	return nil
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

	// Add root handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		indexHTML := `<!DOCTYPE html>
<html lang="ja">
<head>
	<meta charset="UTF-8">
	<title>vpeakserver</title>
	<style>
		body {
			font-family: sans-serif;
			margin: 20px;
			line-height: 1.6;
		}
		h1 {
			font-size: 1.5rem;
			margin-bottom: 1rem;
		}
		.container {
			max-width: 800px;
			margin: 0 auto;
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
		.lang-switch {
			position: absolute;
			top: 20px;
			right: 20px;
			display: flex;
			gap: 10px;
		}
		.lang-switch label {
			margin: initial;
		}
		[data-lang="en"] .ja,
		[data-lang="ja"] .en {
			display: none;
		}
		ul {
			padding-left: 20px;
		}
		li {
			margin: 10px 0;
		}
		a {
			color: #0066cc;
			text-decoration: none;
		}
		a:hover {
			text-decoration: underline;
		}
	</style>
</head>
<body data-lang="{{.Lang}}">
	<div class="lang-switch">
		<label for="langSelect">Language</label>
		<select id="langSelect" onchange="changeLang(this.value)">
			<option value="ja" {{if eq .Lang "ja"}}selected{{end}}>日本語</option>
			<option value="en" {{if eq .Lang "en"}}selected{{end}}>English</option>
		</select>
	</div>

	<div class="container">
		<h1>
			<span class="ja">vpeakserver</span>
			<span class="en">vpeakserver</span>
		</h1>
		<p>
			<span class="ja">vpeakserverへようこそ！</span>
			<span class="en">Welcome to vpeakserver!</span>
		</p>
		<ul>
			<li>
				<a href="/setting">
					<span class="ja">設定</span>
					<span class="en">Settings</span>
				</a>
			</li>
		</ul>
	</div>

	<script>
		function changeLang(lang) {
			document.body.setAttribute('data-lang', lang);
			localStorage.setItem('vpeakserver.selectedLang', lang);
		}

		// initialize language setting
		const savedLang = localStorage.getItem('vpeakserver.selectedLang');
		if (savedLang) {
			document.body.setAttribute('data-lang', savedLang);
			document.getElementById('langSelect').value = savedLang;
		}
	</script>
</body>
</html>`

		tmpl, err := template.New("index").Parse(indexHTML)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
			return
		}

		// Get language preference from localStorage or default to Japanese
		lang := "ja"
		if langCookie, err := r.Cookie("lang"); err == nil {
			lang = langCookie.Value
		}

		data := SettingsData{
			Lang: lang,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/audio_query", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		text := r.URL.Query().Get("text")
		speaker := r.URL.Query().Get("speaker")
		emotion := r.URL.Query().Get("emotion")

		if text == "" || speaker == "" {
			http.Error(w, "Missing required parameters: text and speaker", http.StatusBadRequest)
			return
		}

		speed, err := parseOptionalIntParam(r.URL.Query().Get("speed"), speedMin, speedMax)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid speed parameter: %v", err), http.StatusBadRequest)
			return
		}

		pitch, err := parseOptionalIntParam(r.URL.Query().Get("pitch"), pitchMin, pitchMax)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid pitch parameter: %v", err), http.StatusBadRequest)
			return
		}

		if !validEmotions[emotion] {
			emotion = ""
		}

		audioQuery := AudioQuery{
			Text:    text,
			Speaker: speaker,
			Emotion: emotion,
			Speed:   speed,
			Pitch:   pitch,
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

		if !validEmotions[query.Emotion] {
			query.Emotion = ""
		}

		if err := validateOptionalRange(query.Speed, speedMin, speedMax); err != nil {
			http.Error(w, fmt.Sprintf("Invalid speed: %v", err), http.StatusBadRequest)
			return
		}

		if err := validateOptionalRange(query.Pitch, pitchMin, pitchMax); err != nil {
			http.Error(w, fmt.Sprintf("Invalid pitch: %v", err), http.StatusBadRequest)
			return
		}

		outputFileName := fmt.Sprintf("audio-%s.wav", uuid.New().String())

		opts := vpeak.Options{
			Narrator: query.Speaker,
			Emotion:  query.Emotion,
			Output:   outputFileName,
			Silent:   true,
			Speed:    query.Speed,
			Pitch:    query.Pitch,
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
  <title>vpeakserver Settings</title>
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
      background-color: #fff7d5;
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
    .lang-switch {
      position: absolute;
      top: 20px;
      right: 20px;
    }
    [data-lang="en"] .ja,
    [data-lang="ja"] .en {
      display: none;
    }
  </style>
</head>
<body data-lang="{{.Lang}}">
  <div class="lang-switch" style="display: flex; gap: 10px;">
    <label for="langSelect" style="margin: initial;">Language</label>
    <select id="langSelect" onchange="changeLang(this.value)">
      <option value="ja" {{if eq .Lang "ja"}}selected{{end}}>日本語</option>
      <option value="en" {{if eq .Lang "en"}}selected{{end}}>English</option>
    </select>
  </div>

  <h1>
    <span class="ja">vpeakserver 設定</span>
    <span class="en">vpeakserver Settings</span>
  </h1>

  <div class="alert">
    <span class="ja">変更は即座に反映されます。</span>
    <span class="en">Changes are applied immediately.</span>
  </div>

  <div id="successMessage" class="success-message">
    <span class="ja">設定が保存されました。</span>
    <span class="en">Settings saved.</span>
  </div>

  <form id="settingsForm">
    <label for="corsPolicyMode">CORS Policy Mode</label>
    <select id="corsPolicyMode" name="corsPolicyMode">
      <option value="localapps" {{if eq .CorsPolicyMode "localapps"}}selected{{end}}>localapps</option>
      <option value="all" {{if eq .CorsPolicyMode "all"}}selected{{end}}>all</option>
    </select>
    <div class="description">
      <span class="ja">
        <strong>localapps</strong> はオリジン間リソース共有ポリシーを、
        <code>app://</code> と <code>localhost</code> 関連に限定します。<br>
        その他のオリジンは <strong>Allow Origin</strong> オプションで追加できます。<br>
        <strong>all</strong> はすべてを許可します。危険性を理解した上でご利用ください。
      </span>
      <span class="en">
        <strong>localapps</strong> restricts CORS policy to <code>app://</code> and <code>localhost</code> related origins.<br>
        Additional origins can be added using the <strong>Allow Origin</strong> option.<br>
        <strong>all</strong> allows all origins. Please use with caution.
      </span>
    </div>

    <label for="allowOrigin">Allow Origin</label>
    <input id="allowOrigin" name="allowOrigin" type="text" 
           value="{{.AllowOrigin}}">
    <div class="description">
      <span class="ja">許可するオリジンを指定します。スペースで区切ることで複数指定できます。</span>
      <span class="en">Specify allowed origins. Multiple origins can be specified by separating with spaces.</span>
    </div>
  </form>

  <script>
    document.getElementById('corsPolicyMode').addEventListener('change', saveSettings);
    document.getElementById('allowOrigin').addEventListener('blur', saveSettings);

    function changeLang(lang) {
      document.body.setAttribute('data-lang', lang);
      localStorage.setItem('vpeakserver.selectedLang', lang);
    }

    // initialize language setting
    const savedLang = localStorage.getItem('vpeakserver.selectedLang');
    if (savedLang) {
      document.body.setAttribute('data-lang', savedLang);
      document.getElementById('langSelect').value = savedLang;
    }

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
        const lang = document.body.getAttribute('data-lang');
        console.error(lang === 'ja' ? '設定の保存中にエラーが発生しました:' : 'Error saving settings:', error);
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

			// Get language preference from cookie or default to Japanese
			lang := "ja"
			if langCookie, err := r.Cookie("lang"); err == nil {
				lang = langCookie.Value
			}

			data := SettingsData{
				CorsPolicyMode: corsPolicyMode,
				AllowOrigin:    allowedOrigin,
				Lang:           lang,
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
