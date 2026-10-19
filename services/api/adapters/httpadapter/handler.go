package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"southpark-messenger/services/api/internal/domain"
	"southpark-messenger/services/api/internal/ports"
)

const (
	maxRequestBodyBytes = 1 << 20 // 1 MiB
	publishTimeout      = 5 * time.Second
)

type Handler struct {
	messages ports.MessageUseCase
	logger   *slog.Logger
}

type messageRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(messages ports.MessageUseCase, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{messages: messages, logger: logger}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/messages", h.handleMessages)
	mux.HandleFunc("/healthz", h.handleHealth)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "Content-Type must be application/json"})
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	var request messageRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}

	if err := ensureSingleJSONValue(decoder); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "request body must contain one JSON object"})
		return
	}

	publishContext, cancel := context.WithTimeout(r.Context(), publishTimeout)
	defer cancel()

	if err := h.messages.Send(publishContext, request.Author, request.Body); err != nil {
		switch {
		case errors.Is(err, domain.ErrAuthorRequired), errors.Is(err, domain.ErrBodyRequired):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		default:
			h.logger.Error("could not queue message", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "message broker unavailable"})
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
