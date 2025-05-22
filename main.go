package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// This struct represents a request to the server to create a new license plate entry.
// It can also be used to save the license plate to the DB.
type LicensePlateRequest struct {
	// make it compatible with Gorm
	gorm.Model
	// license plate number received from OCR
	Plate string `json:"plate"`
	// this is the timestamp when we detected license plate
	Timestamp string `json:"timestamp"`
	// hostname of PC
	Hostname string `json:"hostname"`
}

// Capital letters make the variable public
var Log *slog.Logger

func main() {
	Log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// load env vars
	if err := os.Getenv("KAMERAFYR_SERVER_NTFY_URL"); err != "" {
		Log.Error("could not load env var", "error", err)
		return
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		Log.Info("received request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		json := `{"message": "Welcome to the Kamerafyr server!"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(json))
	})

	mux.HandleFunc("POST /licenseplate", func(w http.ResponseWriter, r *http.Request) {
		Log.Info("received request", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)
		err := r.ParseForm()

		if err != nil {
			Log.Error("could not parse form", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr, "error", err)
			http.Error(w, `{"error": "could not parse form"}`, http.StatusBadRequest)
			return
		}

		requestData := LicensePlateRequest{
			Plate:     r.FormValue("plate"),
			Timestamp: r.FormValue("timestamp"),
			Hostname:  r.FormValue("hostname"),
		}

		json, err := json.Marshal(map[string]string{
			"plate":     requestData.Plate,
			"timestamp": requestData.Timestamp,
			"hostname":  requestData.Hostname,
		})

		if err != nil {
			Log.Error("could not marshal json", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr, "error", err)
			http.Error(w, `{"error": "could not marshal json"}`, http.StatusInternalServerError)
			return
		}

		Log.Info("received license plate", "plate", requestData.Plate, "timestamp", requestData.Timestamp, "hostname", requestData.Hostname)

		// Check if similar license plate already exists based on timestamp
		var existingRequest LicensePlateRequest
		err = Database.Where("plate = ? AND timestamp = ?", requestData.Plate, requestData.Timestamp).First(&existingRequest).Error
		if err != gorm.ErrRecordNotFound {
			Database.Delete(&existingRequest)
		}
		if err == nil {
			Log.Info("similar license plate already exists", "plate", requestData.Plate, "timestamp", requestData.Timestamp, "hostname", requestData.Hostname)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "similar license plate already exists"}`, http.StatusBadRequest)
			return
		}

		// Check if license plate already exists, return the existing one
		err = Database.Where("plate = ?", requestData.Plate).First(&existingRequest).Error
		if err != gorm.ErrRecordNotFound {
			// Compare current timestamp with existing one and get the seconds
			existingTime, err1 := time.Parse("2006-01-02T15:04:05.000-07:00", existingRequest.Timestamp)
			currentTime, err2 := time.Parse("2006-01-02T15:04:05.000-07:00", requestData.Timestamp)
			if err1 != nil || err2 != nil {
				Log.Error("invalid timestamp format", "existing", existingRequest.Timestamp, "current", requestData.Timestamp)
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error": "invalid timestamp format"}`, http.StatusBadRequest)
				return
			}
			diff := currentTime.Sub(existingTime).Seconds()
			Log.Info("timestamp difference", "seconds", diff, "plate", requestData.Plate)
			if diff > 300 {
				// License plate is older than 5 minutes, delete the old one.
				Database.Delete(&existingRequest)
				Log.Info("deleted old license plate", "plate", requestData.Plate, "timestamp", existingRequest.Timestamp)
			} else {
				// License plate exists and is not older than 5 minutes. Ok, we need to get the km/h based on 25 metres distance and the seconds between the timestamps.
				kmh := (diff / 25) * 3600
				// If the km/h is above 30, then it's time to give them a fine!
				if kmh > 30 {
					FinePerson(requestData.Plate, kmh)
					Log.Info("license plate is speeding", "plate", requestData.Plate, "kmh", kmh)
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"message": "license plate is speeding"}`, http.StatusBadRequest)
					return
				} else {
					Log.Info("license plate is not speeding", "plate", requestData.Plate, "kmh", kmh)
					w.Header().Set("Content-Type", "application/json")
					http.Error(w, `{"message": "license plate is not speeding"}`, http.StatusBadRequest)
					return
				}
			}
		}

		// Save license plate data to database
		Database.Create(&requestData)
		Log.Info("saved license plate to database", "plate", requestData.Plate, "timestamp", requestData.Timestamp, "hostname", requestData.Hostname)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(json)
	})

	Log.Info("initializing database")
	// TODO: Add error handling maybe?
	InitializeDb()

	Log.Info("starting server", "port", ":8080")
	http.ListenAndServe(":8080", mux)
}

func FinePerson(plate string, kmh float64) {
	// Send notification to my ntfy instance
	url := os.Getenv("KAMERAFYR_SERVER_NTFY_URL")
	req, err := http.NewRequest("POST", url, strings.NewReader("License plate "+plate+" is going "+strconv.FormatFloat(kmh, 'f', 2, 64)+" km/h. Please send them a fine!"))
	if err != nil {
		Log.Error("couldn't send notification to ntfy", "error", err)
		return
	}
	req.Header.Set("Title", "A car is speeding!")
	req.Header.Set("Priority", "4")
	req.Header.Set("Tags", "rotating_light, policeman")
	http.DefaultClient.Do(req)
	Log.Info("sent notification to ntfy", "plate", plate, "kmh", kmh)
}
