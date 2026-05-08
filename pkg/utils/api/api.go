package api

import (
	"encoding/json"
	"net/http"
)

type HTTPResp[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

func SendJSON[T any](w http.ResponseWriter, statusCode int, status string, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(HTTPResp[T]{Status: status, Data: data})
}
