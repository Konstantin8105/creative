package webserver

import (
	"log"
	"net/http"
	"time"

	"github.com/Konstantin8105/creative"
)

// Start launches the web server and blocks until it exits.
func Start(prv *creative.RouterAI, tools []creative.Tool, port string, mode creative.Mode) {
	sm := NewSessionManager(1*time.Hour, func() *creative.Chat {
		newChat := creative.NewChat(prv)
		newChat.AddSystem(mode.GetPrompt())
		newChat.SetTools(tools)
		newChat.AddSystem(creative.ToolsPrompt(tools))
		return newChat
	})
	defer sm.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChat(w, r, sm)
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		handleConfig(w, r, mode)
	})

	addr := ":" + port
	log.Printf("  ?? Web server started at http://localhost%s", addr)
	log.Printf("  ?? Share your local IP with others in your network")
	log.Printf("  ?  Mode: %s", mode.String())
	log.Printf("  ?  Sessions expire after 1 hour of inactivity")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
