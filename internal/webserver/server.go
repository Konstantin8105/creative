package webserver

import (
	"log"
	"net/http"
	"time"

	"github.com/Konstantin8105/creative"
)

// Start launches the web server and blocks until it exits.
func Start(cfg *creative.Config, port string) {
	configDir := cfg.ConfigDir()
	sm := NewSessionManager(cfg, configDir, 1*time.Hour)
	defer sm.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		handleConfig(w, r, cfg)
	})
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, sm)
	})
	mux.HandleFunc("/api/tabs/create", func(w http.ResponseWriter, r *http.Request) {
		handleTabsCreate(w, r, sm)
	})
	mux.HandleFunc("/api/tabs/list", func(w http.ResponseWriter, r *http.Request) {
		handleTabsList(w, r, sm)
	})
	mux.HandleFunc("/api/tabs/close", func(w http.ResponseWriter, r *http.Request) {
		handleTabsClose(w, r, sm)
	})
	mux.HandleFunc("/api/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		handleHeartbeat(w, r, sm)
	})
	mux.HandleFunc("/api/session/close", func(w http.ResponseWriter, r *http.Request) {
		handleSessionClose(w, r, sm)
	})

	addr := ":" + port
	log.Printf("  🌐 Web server started at http://localhost%s", addr)
	log.Printf("  📡 Share your local IP with others in your network")
	log.Printf("  📋 Modes: %d configured", len(cfg.Modes))

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
