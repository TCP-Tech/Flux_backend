package submission_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (s *SubmissionService) Submit(
	ctx context.Context,
	req SubmissionRequest,
) error {
	// get user from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// get the problem
	problem, err := s.ProblemService.GetProblemByID(ctx, req.ProblemID)
	if err != nil {
		return err
	}

	// if the contest is not nil, ask the contest if the user is
	// able to submit to the contest
	if req.ContestID != nil {
		err := s.ContestService.CanSubmit(ctx, req.ProblemID, *req.ContestID)
		if err != nil {
			return err
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
				"error occurred while checking if the problem with id %v is part of ongoing public contest",
				req.ProblemID,
			),
		)
		return err
	}
	if !canSubmitProblemInPractice {
		logrus.Warnf(
			"user %s tried to submit solution problem with id %v in practice while its part of an ongoing contest",
			claims.UserName,
			req.ProblemID,
		)
		return flux_errors.ErrUnAuthorized
	}

	// get the respected evaluator
	evaluator, ok := s.EvaluatorMails[problem.Evaluator]
	if !ok {
		err = fmt.Errorf(
			"%w, no evaluator found with key %s",
			flux_errors.ErrInternal,
			problem.Evaluator,
		)
		logrus.Error(err)
		return err
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
		return err
	}

	// insert into db
	dbSub, err := s.DB.InsertSubmission(
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
		return err
	}

	// prepare the submission struct
	fluxSub, err := dbSubmissionToFluxSubmission(dbSub)
	if err != nil {
		// the submission will be evaluated later. we just inform the user
		// that their submission has been inserted into db successfully
		logrus.Errorf(
			"failed to convert db submission to flux submission. cannot evaluate submission with id %v",
			dbSub.ID,
		)
	}

	// mail the evaluator about the submission
	s.Postman.postMail(
		mail{
			from:     mailSubmissionService,
			to:       evaluator.getMailID(),
			body:     fluxSub,
			priority: evaluator.getSubmissionMailPriority(),
		},
	)

	return nil
}
