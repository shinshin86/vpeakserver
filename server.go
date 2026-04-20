package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/shinshin86/vpeak"
)

const (
	speedMin = 50
	speedMax = 200
	pitchMin = -300
	pitchMax = 300
)

type serverConfig struct {
	allowedOrigin  string
	corsPolicyMode string
	userDictPath   string
}

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

type Server struct {
	mux    *http.ServeMux
	logger *log.Logger

	configMu       sync.RWMutex
	allowedOrigin  string
	corsPolicyMode string
	userDictPath   string
}

func NewServer(cfg serverConfig, logger *log.Logger) *Server {
	s := &Server{
		mux:            http.NewServeMux(),
		logger:         logger,
		allowedOrigin:  cfg.allowedOrigin,
		corsPolicyMode: cfg.corsPolicyMode,
		userDictPath:   cfg.userDictPath,
	}

	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/audio_query", s.enableCORS(s.handleAudioQuery))
	s.mux.HandleFunc("/synthesis", s.enableCORS(s.handleSynthesis))
	s.mux.HandleFunc("/user_dict", s.enableCORS(s.handleUserDict))
	s.mux.HandleFunc("/user_dict_word", s.enableCORS(s.handleCreateUserDictWord))
	s.mux.HandleFunc("/user_dict_word/by-surface/", s.enableCORS(s.handleUserDictWordBySurface))
	s.mux.HandleFunc("/import_user_dict", s.enableCORS(s.handleImportUserDict))
	s.mux.HandleFunc("/setting", s.handleSettings)
	s.mux.HandleFunc("/update-settings", s.handleUpdateSettings)
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

func validateEmotionOption(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	if _, err := vpeak.ParseEmotion(raw); err != nil {
		return "", err
	}

	return raw, nil
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

func (s *Server) enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		s.configMu.RLock()
		corsPolicyMode := s.corsPolicyMode
		allowedOrigin := s.allowedOrigin
		s.configMu.RUnlock()

		if corsPolicyMode == "all" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if corsPolicyMode == "localapps" {
			if strings.HasPrefix(origin, "app://") || strings.HasPrefix(origin, "http://localhost") || origin == allowedOrigin || containsOrigin(allowedOrigin, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	data := SettingsData{Lang: readLanguage(r)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) handleAudioQuery(w http.ResponseWriter, r *http.Request) {
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

	emotion, err = validateEmotionOption(emotion)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid emotion parameter: %v", err), http.StatusBadRequest)
		return
	}

	audioQuery := AudioQuery{
		Text:    text,
		Speaker: speaker,
		Emotion: emotion,
		Speed:   speed,
		Pitch:   pitch,
	}

	writeJSON(w, http.StatusOK, audioQuery)
}

func (s *Server) handleSynthesis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var query AudioQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusBadRequest)
		return
	}

	validatedEmotion, err := validateEmotionOption(query.Emotion)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid emotion: %v", err), http.StatusBadRequest)
		return
	}
	query.Emotion = validatedEmotion

	if err := validateOptionalRange(query.Speed, speedMin, speedMax); err != nil {
		http.Error(w, fmt.Sprintf("Invalid speed: %v", err), http.StatusBadRequest)
		return
	}

	if err := validateOptionalRange(query.Pitch, pitchMin, pitchMax); err != nil {
		http.Error(w, fmt.Sprintf("Invalid pitch: %v", err), http.StatusBadRequest)
		return
	}

	outputFileName := fmt.Sprintf("audio-%s.wav", uuid.NewString())

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
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpl, err := template.New("settings").Parse(settingsHTML)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}

	s.configMu.RLock()
	data := SettingsData{
		CorsPolicyMode: s.corsPolicyMode,
		AllowOrigin:    s.allowedOrigin,
		Lang:           readLanguage(r),
	}
	s.configMu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var settings SettingsData
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request body: %v", err), http.StatusBadRequest)
		return
	}

	s.configMu.Lock()
	s.corsPolicyMode = settings.CorsPolicyMode
	s.allowedOrigin = settings.AllowOrigin
	s.configMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) resolveUserDictPath() (string, error) {
	s.configMu.RLock()
	path := s.userDictPath
	s.configMu.RUnlock()

	if path != "" {
		return path, nil
	}

	return vpeak.DefaultDictionaryPath()
}

func readLanguage(r *http.Request) string {
	lang := "ja"
	if langCookie, err := r.Cookie("lang"); err == nil {
		lang = langCookie.Value
	}
	return lang
}

const indexHTML = `<!DOCTYPE html>
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

		const savedLang = localStorage.getItem('vpeakserver.selectedLang');
		if (savedLang) {
			document.body.setAttribute('data-lang', savedLang);
			document.getElementById('langSelect').value = savedLang;
		}
	</script>
</body>
</html>`

const settingsHTML = `<!DOCTYPE html>
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
