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
	msg := h.service.GetMessage()
	fmt.Fprint(w, msg)
}
