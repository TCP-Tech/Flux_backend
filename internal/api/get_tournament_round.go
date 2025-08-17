package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

func (a *Api) HandlerGetTournamentRound(w http.ResponseWriter, r *http.Request) {
	// get uuid
	tournamentIDStr := r.URL.Query().Get("tournament_id")
	roundNumberStr := r.URL.Query().Get("round_number")

	// parse
	tournamentID, err := uuid.Parse(tournamentIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	roundNumber, err := strconv.Atoi(roundNumberStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get the tournament
	tournamentRound, contests, err := a.TournamentServiceConfig.GetTournamentRound(
		r.Context(),
		tournamentID,
		int32(roundNumber),
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// prepare the response
	response := struct {
		TournamentRound tournament_service.TournamentRound `json:"tournament_round"`
		Contests        []contest_service.Contest          `json:"contests"`
	}{
		tournamentRound, contests,
	}

	// marshal
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", response, err.Error())
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}
