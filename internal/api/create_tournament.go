package api

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

func (a *Api) HandlerCreateTournament(w http.ResponseWriter, r *http.Request) {
	// parse from body
	var tournament tournament_service.Tournament
	err := decodeJsonBody(r.Body, &tournament)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create tournament using service
	serviceTournament, err := a.TournamentServiceConfig.CreateTournament(r.Context(), tournament)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(serviceTournament)
	if err != nil {
		logrus.Errorf("cannot marshal %v, %v", serviceTournament, err)
		http.Error(w, "tournament created but error in preparing response", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusCreated, response)
}
