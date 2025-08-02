package api

import (
	"encoding/json"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const (
	TimeLayout = "2006-01-02 15:04:05"
)

func respondWithJson(w http.ResponseWriter, code int, payload []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(payload)
}

func decodeJsonBody(body io.ReadCloser, params any) error {
	decoder := json.NewDecoder(body)
	err := decoder.Decode(params)
	if err != nil {
		log.Errorf("couldn't decode json body, %v", err)
	}
	return err
}

func (a *Api) HandlerReadiness(w http.ResponseWriter, r *http.Request) {
	respondWithJson(w, http.StatusOK, []byte("Working fine"))
}
