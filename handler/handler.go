package handler

import (
	"fmt"
	"go-project/service"
	"net/http"
)

type TestHandler struct {
	service service.TestService
}

func NewTestHandler(service service.TestService) *TestHandler {
	return &TestHandler{service: service}
}

func (h *TestHandler) Test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	msg := h.service.GetMessage()
	fmt.Fprint(w, msg)
}
