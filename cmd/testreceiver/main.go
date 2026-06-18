package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type receivedPayload struct {
	Text     string `json:"text"`
	FullText string `json:"full_text"`
	Source   string `json:"source"`
	SentAt   string `json:"sent_at"`
}

func main() {
	logPath := "work/test-receiver.log"
	_ = os.MkdirAll("work", 0755)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /receive", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
			return
		}

		var payload receivedPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		entry := map[string]any{
			"received_at": time.Now().UTC().Format(time.RFC3339),
			"payload":     payload,
		}
		line, _ := json.Marshal(entry)
		if file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			_, _ = file.Write(append(line, '\n'))
			_ = file.Close()
		}

		log.Printf("received text=%q full_text=%q source=%q", payload.Text, payload.FullText, payload.Source)
		writeJSON(w, http.StatusOK, map[string]any{
			"received": true,
			"text":     payload.Text,
		})
	})

	server := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("test receiver listening on :8081")
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
