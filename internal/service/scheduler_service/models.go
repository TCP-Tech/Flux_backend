package scheduler_service

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	sigtermWaitDuration = 2
)

type CmdExecType int

const (
	CmdRun CmdExecType = iota
	CmdOutput
	CmdCombined
	CmdLongRunning
)

type TaskState int

const (
	StateQueued TaskState = iota
	StateRunning
	StateCompleted
	StateFailed
	StateKilled

	// Use the below ones with caution
	StateDead // use when you cant determine completed, failed or killed
	StateUnknown
)

type Resources struct {
	CPU    int32
	Memory int32
}

type Scheduler struct {
	Resources       Resources
	QueueBuffer     int32
	resourceRelease chan Resources
	taskQueue       chan uuid.UUID
	tasks           map[uuid.UUID]*Task
	// scheduler's resources and tasks map use the below lock
	taskMapAndResourceLock sync.RWMutex
}

type Command struct {
	Name        string
	Args        []string
	CmdExecType CmdExecType
}

type TaskRequest struct {
	Name              string
	Resources         Resources
	Command           Command
	Priority          int32
	SchedulingRetries int32
	OnLaunchComplete  func(response TaskResponse)
	OnTaskComplete    func(taskID uuid.UUID, err error) // valid only for CmdLongRunning mode tasks
}

type TaskResponse struct {
	TaskID uuid.UUID
	Out    []byte
	Error  error
}

type Task struct {
	TaskRequest
	sync.Mutex
	TaskID          uuid.UUID
	CancelFunc      context.CancelFunc // valid only for CmdLongRunning mode tasks
	cmd             *exec.Cmd
	errorChan       chan error
	QueueTime       time.Time
	LaunchTime      time.Time
	State           TaskState
	SchedulingTries int32
}

func (t Task) String() string {
	return fmt.Sprintf(
		"[TaskID=%s QueueTime=%s LaunchTime=%s State=%v SchedulingTries=%d]",
		t.TaskID, t.QueueTime, t.LaunchTime, t.State, t.SchedulingTries,
	)
}
