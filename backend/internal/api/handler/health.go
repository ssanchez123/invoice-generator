package handler

import (
	"context"
	"log"
	"net/http"
	"time"
)

type readinessChecker interface {
	Ping(context.Context) error
}

func ReadinessHandler(checker readinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := checker.Ping(ctx); err != nil {
			log.Printf("readiness check failed: %v", err)
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
