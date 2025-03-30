package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Payload represents the structure of the incoming JSON payload
type Payload map[string]interface{}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	file, err := os.OpenFile("hooks.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("%s - %s\n", timestamp, string(body))

	if _, err := file.WriteString(entry); err != nil {
		http.Error(w, "Failed to write to file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Payload received and stored."))
}

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	port := ":8080"
	fmt.Println("Webhook server running on port", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
}
