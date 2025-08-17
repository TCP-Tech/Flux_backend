package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

func (a *Api) HandlerGetTournament(w http.ResponseWriter, r *http.Request) {
	// get uuid
	tournamentIDStr := r.URL.Query().Get("tournament_id")

	// parse
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get the tournament
	tournament, err := a.TournamentServiceConfig.GetTournamentByID(r.Context(), tournamentID)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(tournament)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", tournament, err.Error())
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}

func (a *Api) HandlerGetTournamentsByFilters(w http.ResponseWriter, r *http.Request) {
	// parse the request
	var request tournament_service.GetTournamentRequest
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get the tournaments
	tournaments, err := a.TournamentServiceConfig.GetTournamentByFitlers(r.Context(), request)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(tournaments)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", tournaments, err.Error())
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
