package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-project/service"
	"io"
	"net/http"
	"strings"
)

type TestHandler struct {
	service service.TestService
}

func NewTestHandler(service service.TestService) *TestHandler {
	return &TestHandler{service: service}
}

func (h *TestHandler) Test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	msg := h.service.GetMessage()
	fmt.Fprint(w, msg)
}

func (h *TestHandler) DBTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	body := strings.TrimSpace(string(bodyBytes))
	if body == "" {
		http.Error(w, "request body is empty", http.StatusBadRequest)
		return
	}

	record, err := h.service.SaveDBTest(r.Context(), body)
	if err != nil {
		http.Error(w, "failed to save body to database", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, record)
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *TestHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	user, err := h.service.RegisterUser(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *TestHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	response, err := h.service.LoginUser(r.Context(), req.Login, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrUserAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, service.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
