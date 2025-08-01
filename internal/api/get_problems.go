package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (a *Api) HandlerGetProblems(w http.ResponseWriter, r *http.Request) {
	// get problem id
	problemIdStr := r.URL.Query().Get("problem_id")
	if problemIdStr != "" {
		// cast the problemId to int
		problemId, err := strconv.Atoi(problemIdStr)
		if err != nil {
			http.Error(w, "invalid problem id, problem id must be an integer", http.StatusBadRequest)
			return
		}

		a.getProblemById(int32(problemId), w, r.Context())
		return
	}

	// fetch titleSubStr
	title := r.URL.Query().Get("title")

	// Get query parameters for page and pageSize
	page := r.URL.Query().Get("page")
	pageSize := r.URL.Query().Get("page_size")

	// extract the userName or rollNo
	userName := r.URL.Query().Get("user_name")
	rollNo := r.URL.Query().Get("roll_no")

	a.listProblemsWithPagination(
		page, pageSize,
		title, userName, rollNo,
		w, r.Context(),
	)
}

func (a *Api) getProblemById(problemId int32, w http.ResponseWriter, ctx context.Context) {
	// fetch the problem using service
	problem, err := a.ProblemServiceConfig.GetProblemById(ctx, int32(problemId))
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal the response
	responseBytes, err := json.Marshal(problem)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", responseBytes, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}

// Handler function to fetch problems with pagination
func (a *Api) listProblemsWithPagination(
	page, pageSize, title, userName, rollNo string,
	w http.ResponseWriter,
	ctx context.Context,
) {
	problems, err := a.ProblemServiceConfig.ListProblemsWithPagination(
		ctx,
		page, pageSize,
		title, userName, rollNo,
	)
	var statusCode int
	if err != nil {
		if !errors.Is(err, flux_errors.ErrPartialResult) {
			handlerError(err, w)
			return
		}
		statusCode = http.StatusPartialContent
	} else {
		statusCode = http.StatusOK
	}

	// Marshal and respond with the problems
	responseBytes, marshalErr := json.Marshal(problems)
	if marshalErr != nil {
		log.Errorf("unable to marshal problems: %v", marshalErr)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, statusCode, responseBytes)
}
