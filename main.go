package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Payload represents the structure of the incoming JSON payload
type Payload map[string]interface{}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Set permissive CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")   // Allow all origins
	w.Header().Set("Access-Control-Allow-Methods", "*")  // Allow all methods
	w.Header().Set("Access-Control-Allow-Headers", "*")  // Allow all headers
	w.Header().Set("Access-Control-Expose-Headers", "*") // Expose all response headers

	// Handle preflight (OPTIONS) requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close() // Ensure body is closed

	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Open file for appending, creating if it doesn't exist
	file, err := os.OpenFile("hooks.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Format and log entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		http.Error(w, "Failed to format JSON", http.StatusInternalServerError)
		return
	}

	logEntry := fmt.Sprintf("%s - %s\n", timestamp, string(entry))
	if _, err := file.WriteString(logEntry); err != nil {
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
