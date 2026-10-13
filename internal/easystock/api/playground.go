package api

import (
	"encoding/json"
	"html/template"
	"net/http"
)

// playgroundPage is the GraphQL Playground UI. The config object is emitted as
// raw JSON so the endpoint and default headers are not HTML-escaped.
var playgroundPage = template.Must(template.New("playground").Parse(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{.title}}</title>
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/static/css/index.css" />
	<link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/favicon.png" />
	<script src="https://cdn.jsdelivr.net/npm/graphql-playground-react@1.7.26/build/static/js/middleware.js"></script>
	<style>html, body, #root { height: 100%; margin: 0; }</style>
</head>
<body>
	<div id="root"></div>
	<script>
		window.addEventListener('load', function () {
			GraphQLPlayground.init(document.getElementById('root'), {{.config}})
		})
	</script>
</body>
</html>
`))

// Playground serves the GraphQL Playground UI for endpoint. Queries still go
// to that endpoint and keep its authentication checks.
func Playground(title, endpoint string) http.Handler {
	config, err := json.Marshal(map[string]any{
		"endpoint": endpoint,
		"settings": map[string]string{"request.credentials": "same-origin"},
		"headers":  map[string]string{"Authorization": "Bearer "},
	})
	if err != nil {
		panic(err)
	}
	data := map[string]any{"title": title, "config": template.JS(config)}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if err := playgroundPage.Execute(w, data); err != nil {
			http.Error(w, "playground", http.StatusInternalServerError)
		}
	})
}
