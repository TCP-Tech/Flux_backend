package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/contest_service"
)

func (a *Api) HandlerGetContestByID(w http.ResponseWriter, r *http.Request) {
	// get the id
	idStr := r.URL.Query().Get("contest_id")

	// parse into uuid
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get the contest object
	contest, err := a.ContestServiceConfig.GetContestByID(r.Context(), id)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(contest)
	if err != nil {
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	// respond
	respondWithJson(w, http.StatusOK, response)
}

func (a *Api) HandlerGetContestProblems(w http.ResponseWriter, r *http.Request) {
	// get the contest id
	contestIDStr := r.URL.Query().Get("contest_id")

	// parse
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get the contest problems from service
	problems, err := a.ContestServiceConfig.GetContestProblems(r.Context(), contestID)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marhsal
	response, err := json.Marshal(problems)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", problems, err.Error())
		http.Error(
			w, "cannot send problems, internal error. please try again later",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}

func (a *Api) HandlerGetContestUsers(w http.ResponseWriter, r *http.Request) {
	// get the contest id
	contestIDStr := r.URL.Query().Get("contest_id")

	// parse
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// fetch users using service
	users, err := a.ContestServiceConfig.GetContestRegisteredUsers(r.Context(), contestID)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(users)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", users, err.Error())
		http.Error(
			w, "cannot send users, internal error. please try again later",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}

func (a *Api) HandlerGetContestsByFilters(w http.ResponseWriter, r *http.Request) {
	// parse the body
	var request contest_service.GetContestRequest
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get contests
	contests, err := a.ContestServiceConfig.GetContestsByFilters(r.Context(), request)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(contests)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", contests, err.Error())
		http.Error(
			w, "cannot send contests, internal error. please try again later",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}

func (a *Api) HandlerGetUserRegisteredContests(w http.ResponseWriter, r *http.Request) {
	// get the page number
	pageNumberStr := r.URL.Query().Get("page_number")
	pageNumber, err := strconv.Atoi(pageNumberStr)
	if err != nil {
		http.Error(w, "invalid page number", http.StatusBadRequest)
		return
	}

	// get page size
	pageSizeStr := r.URL.Query().Get("page_size")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		http.Error(w, "invalid page size", http.StatusBadRequest)
		return
	}

	// validate
	err = service.ValidateInput(
		struct {
			PageNumber int `validate:"min=1,max=10000"`
			PageSize   int `validate:"min=1,max=10000"`
		}{pageNumber, pageSize},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get contests
	contests, err := a.ContestServiceConfig.GetUserRegisteredContests(
		r.Context(), int32(pageNumber), int32(pageSize),
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(contests)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", contests, err.Error())
		http.Error(
			w, "cannot send contests, internal error. please try again later",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
