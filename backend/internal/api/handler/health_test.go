package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/santiagossaa/invoice-generator/internal/api/handler"
)

type readinessStub struct {
	err error
}

func (s readinessStub) Ping(context.Context) error {
	return s.err
}

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "database available", wantStatus: http.StatusOK},
		{name: "database unavailable", err: errors.New("database offline"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ReadinessHandler(readinessStub{err: test.err})(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			if w.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, test.wantStatus)
			}
		})
	}
}
