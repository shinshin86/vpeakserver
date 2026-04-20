package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/shinshin86/vpeak"
)

var version = "dev"

func main() {
	var cfg serverConfig
	var showVersion bool

	flag.StringVar(&cfg.allowedOrigin, "allowed-origin", "", "Set the allowed CORS origin")
	flag.StringVar(&cfg.corsPolicyMode, "cors-policy-mode", "localapps", "Set the CORS policy mode (localapps or all)")
	flag.StringVar(&cfg.userDictPath, "user-dict-path", "", "Set the VOICEPEAK user dictionary file path")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)
	server := NewServer(cfg, logger)

	dictPath := cfg.userDictPath
	if dictPath == "" {
		if defaultPath, err := vpeak.DefaultDictionaryPath(); err == nil {
			dictPath = defaultPath
		}
	}

	fmt.Println("Server started at http://localhost:20202")
	fmt.Printf("Starting server with allowed origin: %s\n", cfg.allowedOrigin)
	fmt.Printf("CORS policy mode: %s\n", cfg.corsPolicyMode)
	fmt.Printf("VOICEPEAK dictionary path: %s\n", dictPath)

	log.Fatal(http.ListenAndServe(":20202", server.Handler()))
}
