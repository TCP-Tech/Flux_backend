package submission_service

import (
	"net"

	"github.com/oleiade/lane"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func NewPriorityQueue[T Comparable](pqType lane.PQType) *PriorityQueue[T] {
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

func (p *postman) postMail(mail mail) {
	p.mailBox <- mail
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
			err = flux_errors.HandleIPCError(err)
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
	switch cfSubState {
	case "FAILED", "OK", "PARTIAL", "COMPILATION_ERROR", "RUNTIME_ERROR", "WRONG_ANSWER",
		"TIME_LIMIT_EXCEEDED", "MEMORY_LIMIT_EXCEEDED", "IDLENESS_LIMIT_EXCEEDED", "SECURITY_VIOLATED",
		"CRASHED", "INPUT_PREPARATION_CRASHED", "CHALLENGED", "SKIPPED", "REJECTED":
		return true
	default:
		return false
	}
}

func leftShift[T any](slc []T) {
	if len(slc) < 1 {
		return
	}

	for i := 0; i < len(slc)-1; i++ {
		slc[i] = slc[i+1]
	}
}
