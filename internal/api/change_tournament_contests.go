package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

func (a *Api) HandlerChangeTournamentContest(w http.ResponseWriter, r *http.Request) {
	type params struct {
		RoundNumber  int32       `json:"round_number"`
		TournamentID uuid.UUID   `json:"tournament_id"`
		ContestIDs   []uuid.UUID `json:"contest_ids"`
	}
	var request params
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// change contests using service
	contests, err := a.TournamentServiceConfig.ChangeTournamentContests(
		r.Context(),
		tournament_service.ChangeTournamentContestsRequest{
			TournamentID: request.TournamentID,
			RoundNumber:  request.RoundNumber,
			ContestIDs:   request.ContestIDs,
		},
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(contests)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", response, err.Error())
		http.Error(w, "contests changed but error in preparing reponse", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
