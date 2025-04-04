package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
)

// Payload represents the structure of the incoming JSON payload using map[string]any.
type Payload map[string]any

func init() {
	// Set the Apex logger to use the CLI handler for readable console output.
	log.SetHandler(cli.New(os.Stdout))
}

// corsMiddleware handles CORS headers and preflight requests.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set permissive CORS headers.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "*")

		// Handle preflight (OPTIONS) requests.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			// Log the preflight request. Extract client IP for context.
			clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				clientIP = r.RemoteAddr
			}
			log.WithField("clientIP", clientIP).Info("Handled preflight (OPTIONS) request")
			return
		}

		// Proceed to the next handler.
		next.ServeHTTP(w, r)
	})
}

// writePayloadToFile handles marshalling the payload and writing it to a file.
func writePayloadToFile(payload Payload) error {
	// Open the file to append logs.
	file, err := os.OpenFile("../hooks.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Error("Failed to open hooks.txt file")
		return err
	}
	defer file.Close()

	// Marshal the payload to JSON.
	entry, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal JSON payload")
		return err
	}

	// Write the JSON payload to the file.
	logEntry := fmt.Sprintf("%s\n", string(entry))
	if _, err := file.WriteString(logEntry); err != nil {
		log.WithError(err).Error("Failed to write log entry to file")
		return err
	}

	return nil
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client IP for logging.
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}

	// Only allow POST requests.
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		log.WithFields(log.Fields{
			"clientIP": clientIP,
			"method":   r.Method,
		}).Warn("Rejected request: Invalid method")
		return
	}

	// Check Content-Type header.
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Unsupported Media Type: Content-Type must be application/json", http.StatusUnsupportedMediaType)
		log.WithFields(log.Fields{
			"clientIP":    clientIP,
			"ContentType": r.Header.Get("Content-Type"),
		}).Warn("Rejected request: Invalid Content-Type")
		return
	}

	defer r.Body.Close() // Ensure body is closed.

	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.WithFields(log.Fields{
			"clientIP": clientIP,
			"error":    err,
		}).Error("Failed to decode JSON payload")
		return
	}

	// Write the payload to file.
	if err := writePayloadToFile(payload); err != nil {
		http.Error(w, "Failed to write payload to file", http.StatusInternalServerError)
		log.WithField("clientIP", clientIP).Error("Error writing payload to file")
		return
	}

	// Log the successful payload receipt.
	log.WithFields(log.Fields{
		"clientIP": clientIP,
		"payload":  payload,
	}).Info("Payload received and stored successfully")

	// Send a JSON response.
	response := map[string]string{"message": "Payload received and stored."}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithFields(log.Fields{
			"clientIP": clientIP,
			"error":    err,
		}).Error("Failed to write JSON response")
	}
}

func main() {
	// Create a mux and wrap the webhookHandler with CORS middleware.
	mux := http.NewServeMux()
	mux.Handle("/webhook", corsMiddleware(http.HandlerFunc(webhookHandler)))

	port := ":8080"
	log.WithField("port", port).Info("Webhook server starting")
	if err := http.ListenAndServe(port, mux); err != nil {
		log.WithError(err).Fatal("Failed to start server")
		os.Exit(1)
	}
}
