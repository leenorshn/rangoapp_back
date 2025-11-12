package main

import (
	"log"
	"net/http"
	"os"
	"rangoapp/database"
	"rangoapp/directives"
	"rangoapp/graph"
	"rangoapp/middlewares"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Connect to MongoDB
	db := database.ConnectDB()
	defer db.Client().Disconnect(nil)

	router := mux.NewRouter()
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
	log.Fatal(http.ListenAndServe(":"+port, router))
}
