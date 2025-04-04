package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
)

// Payload represents the structure of the incoming JSON payload.
type Payload map[string]any

func init() {
	// Set the Apex logger to use the CLI handler for readable console output.
	log.SetHandler(cli.New(os.Stdout))
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Set permissive CORS headers.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")

	// Handle preflight (OPTIONS) requests.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		log.Info("Handled preflight (OPTIONS) request")
		return
	}

	// Only allow POST requests.
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		log.WithField("method", r.Method).Warn("Rejected request: Invalid method")
		return
	}

	// Check Content-Type header.
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Unsupported Media Type: Content-Type must be application/json", http.StatusUnsupportedMediaType)
		log.WithField("Content-Type", r.Header.Get("Content-Type")).Warn("Rejected request: Invalid Content-Type")
		return
	}

	defer r.Body.Close() // Ensure body is closed.

	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.WithError(err).Error("Failed to decode JSON payload")
		return
	}

	// Open the file to append logs.
	file, err := os.OpenFile("../hooks.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		log.WithError(err).Error("Failed to open hooks.txt file")
		return
	}
	defer file.Close()

	// Format and log entry.
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		http.Error(w, "Failed to format JSON", http.StatusInternalServerError)
		log.WithError(err).Error("Failed to marshal JSON payload")
		return
	}

	logEntry := fmt.Sprintf("%s - %s\n", timestamp, string(entry))
	if _, err := file.WriteString(logEntry); err != nil {
		http.Error(w, "Failed to write to file", http.StatusInternalServerError)
		log.WithError(err).Error("Failed to write log entry to file")
		return
	}

	// Log a successful payload receipt with structured fields.
	log.WithFields(log.Fields{
		"timestamp": timestamp,
		"payload":   payload,
	}).Info("Payload received and stored successfully")

	// Send a JSON response after successfully storing the payload.
	response := map[string]string{"message": "Payload received and stored."}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithError(err).Error("Failed to write JSON response")
	}
}

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	port := ":8080"
	log.WithField("port", port).Info("Webhook server starting")
	if err := http.ListenAndServe(port, nil); err != nil {
		log.WithError(err).Fatal("Failed to start server")
		os.Exit(1)
	}
}
