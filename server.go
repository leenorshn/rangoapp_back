package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"rangoapp/database"
	"rangoapp/directives"
	"rangoapp/graph"
	"rangoapp/handlers"
	"rangoapp/middlewares"
	"rangoapp/services"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

const defaultPort = "8080"

func main() {
	// Load .env file if it exists (ignore error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Connect to MongoDB with retry logic
	db := database.ConnectDB()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Client().Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Start health check monitor
	healthCheckInterval := getHealthCheckInterval()
	db.StartHealthCheckMonitor(healthCheckInterval)

	// Start cron jobs for subscription management
	services.StartCronJobs(db)

	// Setup router
	router := mux.NewRouter()

	// CORS configuration - must be defined before routes
	// This ensures OPTIONS preflight requests are handled correctly for all routes
	allowedOrigins := getAllowedOrigins()
	log.Printf("CORS: Allowed origins: %v", allowedOrigins)
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Cache preflight requests for 5 minutes
		Debug:            os.Getenv("CORS_DEBUG") == "true", // Enable CORS debug logging
	})

	// Apply CORS middleware first (before any route matching)
	router.Use(corsHandler.Handler)

	// Health check endpoints (before auth middleware for external monitoring)
	// These endpoints should be accessible without authentication for monitoring tools
	router.HandleFunc("/health", handlers.HealthHandler(db)).Methods("GET", "OPTIONS")
	router.HandleFunc("/health/ready", handlers.ReadinessHandler(db)).Methods("GET", "OPTIONS")
	router.HandleFunc("/health/live", handlers.LivenessHandler()).Methods("GET", "OPTIONS")

	// Apply auth middleware to all other routes
	router.Use(middlewares.AuthMiddleware)

	// Initialize GraphQL
	c := graph.Config{Resolvers: &graph.Resolver{DB: db}}
	c.Directives.Auth = directives.Auth

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(c))
	// Add multiple transports for better compatibility
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})

	// Setup routes
	router.Handle("/", playground.Handler("GraphQL playground", "/query")).Methods("GET", "OPTIONS")
	router.Handle("/query", srv).Methods("GET", "POST", "OPTIONS")

	// Configure HTTP server with timeouts optimized for Cloud Run
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       15 * time.Second,  // Maximum duration for reading the entire request
		WriteTimeout:      15 * time.Second,  // Maximum duration before timing out writes
		IdleTimeout:       60 * time.Second,  // Maximum amount of time to wait for the next request
		ReadHeaderTimeout: 5 * time.Second,   // Amount of time allowed to read request headers
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		log.Printf("GraphQL playground: http://localhost:%s/", port)
		log.Printf("GraphQL endpoint: http://localhost:%s/query", port)
		log.Printf("Health check: http://localhost:%s/health", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// getHealthCheckInterval returns the configured health check interval
func getHealthCheckInterval() time.Duration {
	intervalStr := os.Getenv("HEALTH_CHECK_INTERVAL_SECONDS")
	if intervalStr == "" {
		return 30 * time.Second // Default: check every 30 seconds
	}

	intervalSeconds, err := strconv.Atoi(intervalStr)
	if err != nil || intervalSeconds <= 0 {
		log.Printf("Invalid HEALTH_CHECK_INTERVAL_SECONDS, using default: 30s")
		return 30 * time.Second
	}

	interval := time.Duration(intervalSeconds) * time.Second
	if interval < 10*time.Second {
		log.Printf("HEALTH_CHECK_INTERVAL_SECONDS too small, using minimum: 10s")
		return 10 * time.Second
	}
	if interval > 300*time.Second {
		log.Printf("HEALTH_CHECK_INTERVAL_SECONDS too large, using maximum: 300s")
		return 300 * time.Second
	}

	return interval
}

// getAllowedOrigins returns the list of allowed CORS origins
// Reads from ALLOWED_ORIGINS environment variable (comma-separated)
// Defaults to localhost for development
func getAllowedOrigins() []string {
	originsEnv := os.Getenv("ALLOWED_ORIGINS")
	if originsEnv != "" {
		// Split by comma and trim spaces
		origins := []string{}
		for _, origin := range splitAndTrim(originsEnv, ",") {
			if origin != "" {
				origins = append(origins, origin)
			}
		}
		if len(origins) > 0 {
			return origins
		}
	}

	// Default origins for development
	return []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:8080",
	}
}

// splitAndTrim splits a string by separator and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}
