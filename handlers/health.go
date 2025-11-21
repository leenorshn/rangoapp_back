package handlers

import (
	"encoding/json"
	"net/http"
	"rangoapp/database"
)

// HealthHandler handles health check requests
func HealthHandler(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Perform actual health check
		status := db.CheckHealth()

		w.Header().Set("Content-Type", "application/json")

		// Set HTTP status code based on health
		if status.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(status)
	}
}

// ReadinessHandler checks if the service is ready to accept traffic
func ReadinessHandler(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := db.CheckHealth()

		w.Header().Set("Content-Type", "application/json")

		if status.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "ready",
				"message": "Service is ready to accept traffic",
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "not ready",
				"message": "Service is not ready: " + status.Message,
			})
		}
	}
}

// LivenessHandler checks if the service is alive (doesn't check DB)
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "alive",
			"message": "Service is alive",
		})
	}
}


