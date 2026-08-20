package swagger

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// cachedJSON holds the spec converted to JSON so we only parse the YAML once.
var (
	cachedJSON []byte
	once       sync.Once
	cachedErr  error
)

// specPath is where the openapi.yaml lives at runtime.
const specPath = "docs/openapi.yaml"

// SpecJSONHandler reads the OpenAPI YAML file and serves it as JSON.
// Swagger UI (swaggo/http-swagger) expects a JSON spec at the URL provided
// via httpSwagger.URL(...).
func SpecJSONHandler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		data, err := os.ReadFile(specPath)
		if err != nil {
			cachedErr = err
			return
		}
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			cachedErr = err
			return
		}
		cachedJSON, cachedErr = json.Marshal(raw)
	})

	if cachedErr != nil {
		http.Error(w, "failed to load openapi spec: "+cachedErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(cachedJSON)
}

// SpecYAMLHandler serves the raw YAML file as-is.
func SpecYAMLHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, specPath)
}
