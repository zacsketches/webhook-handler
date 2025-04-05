package main

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	_ "github.com/mattn/go-sqlite3"
)

type Payload map[string]any

var db *sql.DB

func init() {
	log.SetHandler(cli.New(os.Stdout))

	var err error
	db, err = sql.Open("sqlite3", "/mnt/readings/db/measurements.db")
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to SQLite database")
	}
	log.Info("Successfull located database")

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS water_tests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		testDate TEXT NOT NULL,
		chlorine REAL NOT NULL,
		ph REAL NOT NULL,
		acidDemand INTEGER,
		totalAlkalinity INTEGER
	)`)
	if err != nil {
		log.WithError(err).Fatal("Failed to create table")
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "*")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func storePayload(payload Payload) error {
	testDate, _ := payload["testDate"].(string)
	chlorine, _ := payload["chlorine"].(float64)
	ph, _ := payload["ph"].(float64)
	acidDemand, _ := payload["acidDemand"].(float64)           // will convert to int
	totalAlkalinity, _ := payload["totalAlkalinity"].(float64) // will convert to int

	_, err := db.Exec(`
		INSERT INTO water_tests (testDate, chlorine, ph, acidDemand, totalAlkalinity)
		VALUES (?, ?, ?, ?, ?)
	`, testDate, chlorine, ph, int(acidDemand), int(totalAlkalinity))

	return err
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
		return
	}

	defer r.Body.Close()

	var payload Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := storePayload(payload); err != nil {
		http.Error(w, "Failed to store payload", http.StatusInternalServerError)
		log.WithField("clientIP", clientIP).WithError(err).Error("Error storing payload")
		return
	}

	log.WithFields(log.Fields{"clientIP": clientIP}).Info("Payload stored successfully")

	response := map[string]string{"message": "Payload received and stored."}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/webhook", corsMiddleware(http.HandlerFunc(webhookHandler)))

	log.Info("Webhook server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.WithError(err).Fatal("Server failed")
	}
}
