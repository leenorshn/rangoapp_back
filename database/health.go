package database

import (
	"context"
	"os"
	"sync"
	"time"

	"rangoapp/utils"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	healthStatus      = "unknown"
	healthStatusMutex sync.RWMutex
	lastHealthCheck   time.Time
	healthCheckMutex  sync.Mutex
)

// HealthStatus represents the health status of the database connection
type HealthStatus struct {
	Status    string    `json:"status"`    // "healthy", "unhealthy", "unknown"
	Message   string    `json:"message"`   // Human-readable message
	Timestamp time.Time `json:"timestamp"` // Last check timestamp
	Latency   string    `json:"latency"`   // Ping latency in milliseconds
}

// CheckHealth performs a health check on the MongoDB connection
func (db *DB) CheckHealth() HealthStatus {
	healthCheckMutex.Lock()
	defer healthCheckMutex.Unlock()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Ping the database
	err := db.client.Ping(ctx, nil)
	latency := time.Since(start)

	healthStatusMutex.Lock()
	defer healthStatusMutex.Unlock()

	if err != nil {
		healthStatus = "unhealthy"
		lastHealthCheck = time.Now()
		utils.Error("MongoDB health check failed: %v", err)
		return HealthStatus{
			Status:    "unhealthy",
			Message:   "MongoDB connection failed : " + err.Error(),
			Timestamp: lastHealthCheck,
			Latency:   latency.String(),
		}
	}

	healthStatus = "healthy"
	lastHealthCheck = time.Now()
	return HealthStatus{
		Status:    "healthy",
		Message:   "MongoDB connection is healthy",
		Timestamp: lastHealthCheck,
		Latency:   latency.String(),
	}
}

// GetHealthStatus returns the last known health status without performing a check
func GetHealthStatus() HealthStatus {
	healthStatusMutex.RLock()
	defer healthStatusMutex.RUnlock()

	return HealthStatus{
		Status:    healthStatus,
		Message:   getHealthMessage(healthStatus),
		Timestamp: lastHealthCheck,
	}
}

func getHealthMessage(status string) string {
	switch status {
	case "healthy":
		return "MongoDB connection is healthy"
	case "unhealthy":
		return "MongoDB connection is unhealthy"
	default:
		return "MongoDB health status is unknown"
	}
}

// StartHealthCheckMonitor starts a background goroutine that periodically checks MongoDB health
func (db *DB) StartHealthCheckMonitor(interval time.Duration) {
	if interval < 10*time.Second {
		interval = 10 * time.Second // Minimum 10 seconds
		utils.Warning("Health check interval too small, using minimum: 10s")
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		utils.Info("Started MongoDB health check monitor (interval: %v)", interval)

		for range ticker.C {
			status := db.CheckHealth()
			if status.Status == "unhealthy" {
				utils.Error("MongoDB health check detected unhealthy status")
				// Could trigger alerts or reconnection logic here
			} else {
				utils.Debug("MongoDB health check: %s (latency: %s)", status.Status, status.Latency)
			}
		}
	}()
}

// Reconnect attempts to reconnect to MongoDB
func (db *DB) Reconnect() error {
	utils.Info("Attempting to reconnect to MongoDB...")

	ctx, cancel := context.WithTimeout(context.Background(), getConnectTimeout())
	defer cancel()

	// Disconnect existing connection if any
	if db.client != nil {
		_ = db.client.Disconnect(ctx)
	}

	// Get connection parameters
	dbUrl := os.Getenv("MONGO_URI")
	if dbUrl == "" {
		return utils.NewDatabaseError("reconnect", nil)
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "rangodb"
	}

	// Create new client
	client, err := mongo.NewClient(options.Client().ApplyURI(dbUrl))
	if err != nil {
		return utils.WrapError(err, "Failed to create MongoDB client")
	}

	// Connect with retry
	maxRetries := getMaxRetries()
	err = connectWithRetry(client, ctx, maxRetries, 2*time.Second)
	if err != nil {
		return err
	}

	// Update dbInstance
	db.client = client
	db.database = client.Database(dbName)

	utils.Info("Successfully reconnected to MongoDB")
	return nil
}
