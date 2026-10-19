package httpadapter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"southpark-messenger/services/api/internal/domain"
)

type messageUseCaseStub struct {
	author string
	body   string
	err    error
	calls  int
}

func (s *messageUseCaseStub) Send(_ context.Context, author, body string) error {
	s.calls++
	s.author = author
	s.body = body
	return s.err
}

func TestHandlerAcceptsValidMessage(t *testing.T) {
	useCase := &messageUseCaseStub{}
	recorder := performRequest(t, useCase, http.MethodPost, `{"author":"Cartman","body":"Respect my authoritah!"}`, "application/json")

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	if useCase.calls != 1 || useCase.author != "Cartman" || useCase.body != "Respect my authoritah!" {
		t.Fatalf("use case received calls=%d author=%q body=%q", useCase.calls, useCase.author, useCase.body)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestHandlerRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		body        string
		contentType string
		useCaseErr  error
		wantStatus  int
	}{
		{
			name:        "wrong method",
			method:      http.MethodGet,
			contentType: "application/json",
			wantStatus:  http.StatusMethodNotAllowed,
		},
		{
			name:        "wrong content type",
			method:      http.MethodPost,
			body:        `{}`,
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "malformed JSON",
			method:      http.MethodPost,
			body:        `{"author":`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "unknown field",
			method:      http.MethodPost,
			body:        `{"author":"Kenny","body":"Mmph","alive":false}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "multiple values",
			method:      http.MethodPost,
			body:        `{"author":"Kenny","body":"Mmph"} {}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing author",
			method:      http.MethodPost,
			body:        `{"body":"Dude!"}`,
			contentType: "application/json",
			useCaseErr:  domain.ErrAuthorRequired,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "publisher unavailable",
			method:      http.MethodPost,
			body:        `{"author":"Kyle","body":"Dude!"}`,
			contentType: "application/json",
			useCaseErr:  errors.New("broker unavailable"),
			wantStatus:  http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCase := &messageUseCaseStub{err: tt.useCaseErr}
			recorder := performRequest(t, useCase, tt.method, tt.body, tt.contentType)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}

func performRequest(t *testing.T, useCase *messageUseCaseStub, method, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(useCase, logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(method, "/messages", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}
