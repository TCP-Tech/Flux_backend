package submission_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (ssm *subStatManagerImpl) start() {
	ssm.logger = logrus.WithFields(
		logrus.Fields{
			"from": "sub-stat-manager-impl",
		},
	)
}

func (ssm *subStatManagerImpl) getSubmission(ctx context.Context, subID uuid.UUID) (any, error) {
	dbSub, err := ssm.db.GetSubmissionByID(ctx, subID)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot get submission with id %v from db", subID),
		)
		logrus.Error(err)
		return nil, err
	}

	// get the related problem
	problem, err := ssm.problemServiceConfig.GetProblemByID(ctx, dbSub.ProblemID)
	if err != nil {
		return nil, err
	}

	// convert to fluxSub
	fluxSub, err := dbSubmissionToFluxSubmission(dbSub)
	if err != nil {
		return nil, err
	}

	// get submission from db based on evaluator
	switch problem.Evaluator {
	case platformCodeforces:
		return ssm.getCfSubmissionByID(ctx, fluxSub)
	default:
		err = fmt.Errorf(
			"%w, unknown evaluator %v, cannot prepare a submission status response",
			flux_errors.ErrInternal,
			problem.Evaluator,
		)
		logrus.Error(err)
		return nil, err
	}

}

func (ssm *subStatManagerImpl) getCfSubmissionByID(ctx context.Context, fluxSub fluxSubmission) (any, error) {
	var claims *service.UserCredentialClaims
	if v := ctx.Value(internalSubmissionQuery); v == nil {
		var clms service.UserCredentialClaims
		clms, err := service.GetClaimsFromContext(ctx)
		if err != nil {
			return nil, err
		}
		claims = &clms
	}

	// get cf submission from db
	dbCfSub, err := ssm.db.GetCfSubmissionById(ctx, fluxSub.SubmissionID)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot get cf submission with id %v from db", fluxSub.SubmissionID),
		)
		return nil, err
	}

	// prepare the response
	res := dbCfSubStatus{
		fluxSubmission:      fluxSub,
		cfSubID:             dbCfSub.CfSubID,
		TimeConsumedMillis:  dbCfSub.TimeConsumedMillis,
		MemoryConsumedBytes: dbCfSub.MemoryConsumedBytes,
		PassedTestCount:     dbCfSub.PassedTestCount,
	}

	// other users cant see solutions during a contest
	if claims != nil && claims.UserId != fluxSub.SubmittedBy && fluxSub.ContestID != nil {
		contest, err := ssm.contestServiceConfig.GetContestByID(ctx, *fluxSub.ContestID)
		if err != nil {
			logrus.Errorf(
				"failed to get contest with id %v from ContestServiceConfig while deciding to send solution via json response",
				fluxSub.ContestID,
			)
			return nil, err
		}
		if time.Now().Before(contest.EndTime) {
			res.Solution = nil
		}
	}

	return res, nil
}

// used for updating submssion status along with some other table in a transaction
func (ssm *subStatManagerImpl) updateSubmission(
	ctx context.Context,
	qtx *database.Queries,
	subID uuid.UUID,
	state string,
) (fluxSubmission, error) {
	dbSub, err := qtx.UpdateSubmissionByID(
		ctx,
		database.UpdateSubmissionByIDParams{
			ID:    subID,
			State: state,
		},
	)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf(
				"cannot update submission with id %v",
				getShortUUID(subID, 5),
			),
		)
		return fluxSubmission{}, err
	}

	return dbSubmissionToFluxSubmission(dbSub)
}

func (ssm *subStatManagerImpl) bulkUpdateSubmissionState(
	ctx context.Context,
	qtx *database.Queries,
	subIds []uuid.UUID,
	states []string,
) error {
	if len(subIds) != len(states) {
		err := fmt.Errorf(
			"%w, submission ids and states are not equal in length. cannot bulk update submission states",
			flux_errors.ErrInvalidRequest,
		)
		ssm.logger.Error(err)
		return err
	}

	if qtx == nil {
		err := fmt.Errorf(
			"%w, query transaction tool is null. cannot bulk update submission states",
			flux_errors.ErrInvalidRequest,
		)
		ssm.logger.Error(err)
		return err
	}

	if len(subIds) == 0 {
		ssm.logger.Warn("recieved request with 0 length submission ids to bulk update submissions")
		return nil
	}

	err := qtx.BulkUpdateSubmissionState(ctx,
		database.BulkUpdateSubmissionStateParams{Ids: subIds, States: states},
	)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err, errMsgs, "cannot bulk update submission states in db",
		)
		return err
	}

	return nil
}
