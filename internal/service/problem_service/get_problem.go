package problem_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (p *ProblemService) GetProblemById(
	ctx context.Context,
	problem_id int32,
) (problem Problem, err error) {
	// fetch problem from db
	dbProblem, err := p.DB.GetProblemById(ctx, problem_id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("%w, problem with id %v not found", flux_errors.ErrNotFound, problem_id)
			return
		}
		err = fmt.Errorf("%w, failed to fetch problem with id %v, %w", flux_errors.ErrInternal, problem_id, err)
		log.Error(err)
		return
	}
	problem, err = dbProblemToServiceProblem(dbProblem)
	return
}

func (p *ProblemService) ListProblemsWithPagination(
	ctx context.Context,
	pageStr string,
	pageSizeStr string,
	title string,
	userName string,
	rollNo string,
) ([]Problem, error) {
	// Set a default page size and a maximum limit
	var pageSize int32 = 10
	if pageSizeStr != "" {
		parsedSize, err := strconv.Atoi(pageSizeStr)
		if err == nil && parsedSize > 0 {
			pageSize = int32(parsedSize)
		} else {
			err = fmt.Errorf("%w, page_size must be a valid integer", flux_errors.ErrInvalidInput)
			return nil, err
		}
	} else {
		log.Error("pageSize size is not provided, default set to 10")
	}

	// Set a default page number and validate it
	var page int32 = 1
	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err == nil && parsedPage > 0 {
			page = int32(parsedPage)
		} else {
			err = fmt.Errorf("%w, page number must be a valid integer", flux_errors.ErrInvalidInput)
			return nil, err
		}
	} else {
		log.Error("page number is not provided, default set to 1")
	}

	// filter for created_by or last_updated_by
	var authorId uuid.NullUUID
	if userName != "" || rollNo != "" {
		user, err := p.UserServiceConfig.FetchUserFromDb(ctx, userName, rollNo)
		if err != nil {
			return nil, err
		}
		authorId = uuid.NullUUID{
			Valid: true,
			UUID:  user.ID,
		}
	}

	// calculate offset from pageNumber
	offset := (page - 1) * pageSize

	// fetch the problems from db
	dbProblems, err := p.DB.ListProblemsWithPagination(
		ctx,
		database.ListProblemsWithPaginationParams{
			Limit:    pageSize,
			Offset:   offset,
			Title:    fmt.Sprintf("%%%s%%", title),
			AuthorID: authorId,
		},
	)
	if err != nil {
		err = fmt.Errorf("%w, unable to fetch problem with given filters, %w", flux_errors.ErrInternal, err)
		log.WithFields(log.Fields{
			"page":     page,
			"pageSize": pageSize,
			"title":    title,
		}).Error(err)
		return nil, err
	}

	// convert to service problems
	problems, err := dbProblemsToServiceProblems(dbProblems)
	return problems, err
}
