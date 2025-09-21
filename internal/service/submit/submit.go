package submission_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (s *SubmissionService) Submit(
	ctx context.Context,
	req SubmissionRequest,
) (uuid.UUID, error) {
	// get user from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	// get the problem
	problem, err := s.ProblemService.GetProblemByID(ctx, req.ProblemID)
	if err != nil {
		return uuid.Nil, err
	}

	// if the contest is not nil, ask the contest if the user is
	// able to submit to the contest
	if req.ContestID != nil {
		err := s.ContestService.CanSubmit(ctx, req.ProblemID, *req.ContestID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	// check if the problem is part of any ongoing public contest
	canSubmitProblemInPractice, err := s.DB.CanSubmitProblemInPractice(
		ctx,
		req.ProblemID,
	)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf(
				"error occurred if the problem with id %v is part of ongoing public contest",
				req.ProblemID,
			),
		)
		return uuid.Nil, err
	}
	if !canSubmitProblemInPractice {
		logrus.Warnf(
			"user %s tried to submit solution problem with id %v in practice while its part of an ongoing contest",
			claims.UserName,
			req.ProblemID,
		)
		return uuid.Nil, flux_errors.ErrUnAuthorized
	}

	// get the respected evaluator
	evaluator, ok := s.Evaluators[problem.Evaluator]
	if !ok {
		err = fmt.Errorf(
			"%w, no evaluator found with key %s",
			flux_errors.ErrInternal,
			problem.Evaluator,
		)
		logrus.Error(err)
		return uuid.Nil, err
	}

	// marshal solution
	bytes, err := json.Marshal(req.Solution)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			req.Solution,
			err,
		)
		logrus.Error(err)
		return uuid.Nil, err
	}

	// insert into db
	submission, err := s.DB.InsertSubmission(
		ctx,
		database.InsertSubmissionParams{
			SubmittedBy: claims.UserId,
			ContestID:   req.ContestID,
			ProblemID:   req.ProblemID,
			Solution:    json.RawMessage(bytes),
			State:       SubStatusFluxQueued,
		},
	)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf(
				"cannot insert submission to problem with id %v and contest with id %v of user %s in db",
				req.ProblemID,
				req.ContestID,
				claims.UserName,
			),
		)
		return uuid.Nil, err
	}

	// evaluate
	go evaluator.evaluate(submission.ID)

	return submission.ID, nil
}
