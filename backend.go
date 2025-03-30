package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	body, err := ioutil.ReadAll(r.Body)
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

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("webhook_payload_%s.txt", timestamp)

	if err := ioutil.WriteFile(filename, body, 0644); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
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
