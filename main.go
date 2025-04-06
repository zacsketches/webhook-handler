package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	_ "github.com/mattn/go-sqlite3"
)

type Payload map[string]any
type WaterTest struct {
	ID              int     `json:"id"`
	TestDate        string  `json:"testDate"`
	Chlorine        float64 `json:"chlorine"`
	Ph              float64 `json:"ph"`
	AcidDemand      int     `json:"acidDemand"`
	TotalAlkalinity int     `json:"totalAlkalinity"`
}

var db *sql.DB

func init() {
	// Set up logging
	log.SetHandler(cli.New(os.Stdout))

	// Get the READINGS_DB environment variable
	dbPath := os.Getenv("READINGS_DB")
	if dbPath == "" {
		log.Fatal("READINGS_DB environment variable is not set")
	}

	// Check if the given path is a valid Unix file path
	// We use filepath.IsAbs to check if the path is absolute and os.Stat to check if it exists
	if !filepath.IsAbs(dbPath) {
		log.Fatal("The READINGS_DB environment variable must be a valid Unix file path")
	}

	// Check if the file exists at the given path
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.WithField("path", dbPath).Fatal("Database file does not exist")
	}

	// Open the SQLite database connection
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to SQLite database")
	}
	log.Info("Successfully located database")

	// Check if the water_tests table exists by running a simple query
	var tableExists bool
	err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='water_tests'`).Scan(&tableExists)
	if err != nil {
		log.WithError(err).Fatal("Failed to check if water_tests table exists")
	}

	// If the table doesn't exist, create it and log the creation
	if !tableExists {
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS water_tests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			testDate TEXT NOT NULL,
			chlorine REAL NOT NULL,
			ph REAL NOT NULL,
			acidDemand INTEGER,
			totalAlkalinity INTEGER
		)`)
		if err != nil {
			log.WithError(err).Fatal("Failed to create table water_tests")
		}
		log.Info("water_tests table was created")
	} else {
		// If the table exists, count how many readings (rows) are in the table
		var count int
		err = db.QueryRow(`SELECT COUNT(*) FROM water_tests`).Scan(&count)
		if err != nil {
			log.WithError(err).Fatal("Failed to count readings in water_tests table")
		}
		log.WithField("readingsCount", count).Info("water_tests table already exists")
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, POST, GET")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "*")

		// If the request method is OPTIONS, respond with a No Content status (used for preflight requests)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func storePayload(payload Payload) error {
	// Extract fields from the payload and assert as float64
	testDate, _ := payload["testDate"].(string)
	chlorine, _ := payload["chlorine"].(float64)
	ph, _ := payload["ph"].(float64)
	acidDemand, _ := payload["acidDemand"].(float64)           // Assert as float64
	totalAlkalinity, _ := payload["totalAlkalinity"].(float64) // Assert as float64

	// Round the acidDemand and totalAlkalinity to the nearest integer and convert to int
	acidDemandInt := int(math.Round(acidDemand))
	totalAlkalinityInt := int(math.Round(totalAlkalinity))

	// Insert the data into the database
	_, err := db.Exec(`
		INSERT INTO water_tests (testDate, chlorine, ph, acidDemand, totalAlkalinity)
		VALUES (?, ?, ?, ?, ?)
	`, testDate, chlorine, ph, acidDemandInt, totalAlkalinityInt)

	return err
}

func getReadings() ([]WaterTest, error) {
	rows, err := db.Query(`SELECT id, testDate, chlorine, ph, acidDemand, totalAlkalinity FROM water_tests`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var readings []WaterTest

	// Loop through the rows returned from the database
	for rows.Next() {
		var reading WaterTest
		// Scan the current row's values into the WaterTest struct
		if err := rows.Scan(&reading.ID, &reading.TestDate, &reading.Chlorine, &reading.Ph, &reading.AcidDemand, &reading.TotalAlkalinity); err != nil {
			return nil, err
		}
		readings = append(readings, reading)
	}

	// Check if there was an error during iteration (e.g., while fetching rows)
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return readings, nil
}

func readingsHandler(w http.ResponseWriter, r *http.Request) {
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	readings, err := getReadings()
	if err != nil {
		http.Error(w, "Failed to retrieve readings", http.StatusInternalServerError)
		log.WithField("clientIP", clientIP).WithError(err).Error("Error retrieving readings")
		return
	}

	response := map[string]interface{}{
		"readings": readings,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	mux.Handle("/readings", corsMiddleware(http.HandlerFunc(readingsHandler)))

	log.Info("Webhook server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.WithError(err).Fatal("Server failed")
	}
}
