package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"rangoapp/database"
	"rangoapp/directives"
	"rangoapp/graph"
	"rangoapp/handlers"
	"rangoapp/middlewares"
	"time"

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
	defer db.Client().Disconnect(nil)

	// Start health check monitor
	healthCheckInterval := getHealthCheckInterval()
	db.StartHealthCheckMonitor(healthCheckInterval)

	router := mux.NewRouter()

	// Health check endpoints (before auth middleware for external monitoring)
	// These endpoints should be accessible without authentication for monitoring tools
	router.HandleFunc("/health", handlers.HealthHandler(db)).Methods("GET")
	router.HandleFunc("/health/ready", handlers.ReadinessHandler(db)).Methods("GET")
	router.HandleFunc("/health/live", handlers.LivenessHandler()).Methods("GET")

	// Apply auth middleware to all other routes
	router.Use(middlewares.AuthMiddleware)

	c := graph.Config{Resolvers: &graph.Resolver{DB: db}}
	c.Directives.Auth = directives.Auth

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(c))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.Introspection{})

	// CORS configuration
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", corsHandler.Handler(srv))

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Printf("Health check endpoint: http://localhost:%s/health", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
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
