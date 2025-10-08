package submission_service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func NewPriorityQueue[T Prioritizable](pqType lane.PQType) *PriorityQueue[T] {
	return &PriorityQueue[T]{
		inner: lane.NewPQueue(pqType),
	}
}

func (p *PriorityQueue[T]) Add(c T) {
	p.inner.Push(c, c.GetPriority())
}

func (p *PriorityQueue[T]) Size() int {
	return p.inner.Size()
}

// the bool field represent if the returned field is valid or not
func (p *PriorityQueue[T]) Peek() (T, bool) {
	top, _ := p.inner.Head()
	if top == nil {
		var zero T
		return zero, false
	}
	return top.(T), true
}

// the bool field represent if the returned field is valid or not
func (p *PriorityQueue[T]) Pop() (T, bool) {
	top, _ := p.inner.Pop()
	if top == nil {
		var zero T
		return zero, false
	}
	return top.(T), true
}

func (m mail) GetPriority() int {
	return m.priority
}

// conn usually writes complete bytes but the contract allows for writing partial bytes
// so check everytime if all the bytes were written successfully
// always set a deadline before writing. If the reading socket fails to close the connection
// it will block indefinetely
func writeToConn(
	conn net.Conn, data []byte,
) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			err = flux_errors.WrapIPCError(err)
			return err
		}
		total += n
	}
	return nil
}

func (st subTimeStamp) GetPriority() int {
	return int(st)
}

func isCfSubSinkState(cfSubState string) bool {
	for _, state := range cfSinkStates {
		if state == cfSubState {
			return true
		}
	}
	return false
}

func createRandomFile(
	parDir string,
	filePrefix string,
	fileExtension string,
	numRetries int32,
) (string, error) {
	for range numRetries {
		file := fmt.Sprintf(
			"%s/%s__%d.%s",
			parDir,
			filePrefix,
			rand.Int31(),
			fileExtension,
		)

		// Open the file with O_CREATE and O_EXCL flags.
		// This is an atomic operation. It will only succeed if the file
		// does not already exist. If it exists, it returns an error.
		// 0600 ensures only owner can read from or write to the file
		f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			// The file was successfully created, so it's safe to use.
			// Close the file handle immediately
			if err = f.Close(); err != nil {
				logrus.Errorf(
					"error occurred while closing file %v: %v",
					file, err,
				)
			}
			return file, nil
		}

		// Handle the "file already exists" error.
		if os.IsExist(err) {
			// This is not a fatal error; just retry with a new random name.
			continue
		}

		// Handle other, more serious errors.
		logrus.Errorf(
			"error occurred while creating file %v: %v",
			file, err,
		)
	}

	return "", fmt.Errorf(
		"%w, cannot create a file after %d tries",
		flux_errors.ErrInternal,
		numRetries,
	)
}

func getFileExtensionFromLanguage(language string) (string, error) {
	switch language {
	case languageJava:
		return "java", nil
	case languageCpp:
		return "cpp", nil
	case languagePython:
		return "py", nil
	default:
		return "", flux_errors.ErrInvalidRequest
	}
}

func dbSubmissionToFluxSubmission(sub database.Submission) (fluxSubmission, error) {
	// marhsal the solution
	var solution map[string]string
	if err := json.Unmarshal(sub.Solution, &solution); err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			sub.Solution,
			err,
		)
		logrus.Error(err)
		return fluxSubmission{}, err
	}

	return fluxSubmission{
		SubmissionID: sub.ID,
		SubmittedBy:  sub.SubmittedBy,
		ProblemID:    sub.ProblemID,
		ContestID:    sub.ContestID,
		Solution:     solution,
		State:        sub.State,
		SubmitedAt:   sub.SubmittedAt,
		UpdatedAt:    sub.UpdatedAt,
	}, nil
}

// use this carefully. it doesn't check for array index out of bounds for simplicity
func getShortUUID(id uuid.UUID, len int) string {
	return id.String()[:len] + "..."
}

func getRandomComment(language string) (string, error) {
	randomComment := uuid.New()
	switch language {
	case languageJava, languageCpp:
		return "//" + randomComment.String(), nil
	case languagePython:
		return "#" + randomComment.String(), nil
	default:
		err := fmt.Errorf(
			"%w, cannot generate a random comment for invalid language '%v'",
			flux_errors.ErrInvalidRequest,
			language,
		)
		logrus.Error(err)
		return "", err
	}
}

func isNonSinkFluxState(state string) bool {
	for _, st := range nonSinkFluxStates {
		if state == st {
			return true
		}
	}
	return false
}

func getContextWithKeys(
	timeout time.Duration,
	keys ...service.InternalContextKey,
) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	for _, key := range keys {
		ctx = context.WithValue(ctx, key, struct{}{})
	}

	return ctx, cancel
}

func dbBotToFluxBot(dbBot database.Bot) (Bot, error) {
	// unmarhsal cookies
	var cookies map[string]string
	err := json.Unmarshal(dbBot.Cookies, &cookies)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot unmarshal cookies of bot %v, %w",
			// make it invalid so that manager can also understand about the error
			flux_errors.ErrInvalidRequest,
			dbBot.Name,
			err,
		)
		logrus.Error(err)
		return Bot{}, err
	}

	return Bot{
		Name:     dbBot.Name,
		Platform: dbBot.Platform,
		Cookies:  cookies,
	}, nil

}
