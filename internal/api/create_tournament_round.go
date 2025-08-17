package api

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

func (a *Api) HandlerCreateTournamentRound(w http.ResponseWriter, r *http.Request) {
	// parse the request
	var round tournament_service.TournamentRound
	err := decodeJsonBody(r.Body, &round)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create round using service
	serviceRound, err := a.TournamentServiceConfig.CreateTournamentRound(
		r.Context(),
		round,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(serviceRound)
	if err != nil {
		log.Errorf("cannot marhsal %v, %v", serviceRound, err.Error())
		http.Error(w, "round has been created but error preparing response", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusCreated, response)
}
