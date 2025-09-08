package scheduler_service

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (r *Resources) add(other Resources) {
	r.CPU += other.CPU
	r.Memory += other.Memory
}

func (r *Resources) greater(other Resources) bool {
	return r.CPU > other.CPU && r.Memory > other.Memory
}

func (r *Resources) use(other Resources) error {
	if other.greater(*r) {
		return fmt.Errorf(
			"%w, requested resources are greater than available resources",
			flux_errors.ErrInvalidRequest,
		)
	}

	r.CPU -= other.CPU
	r.Memory -= other.Memory

	return nil
}

func sortWithPriorityAndLaunchTime(slice []*Task) {
	sort.Slice(
		slice, func(i, j int) bool {
			// first is low priority
			if slice[i].Priority < slice[j].Priority {
				return true
			}

			// first is high priority
			if slice[i].Priority > slice[j].Priority {
				return false
			}

			// both are same, youngest die first
			if slice[i].LaunchTime.After(slice[j].LaunchTime) {
				return true
			}

			return false
		},
	)
}

func (s *Scheduler) killTasks(tasksToKill ...*Task) {
	// create a waitgroup to count how many tasks have been killed
	wg := sync.WaitGroup{}

	for _, task := range tasksToKill {
		wg.Add(1)
		go s.killTask(task, &wg)
	}

	wg.Wait()
}

// this function can be used to launch as go-routine or can be called directly
func (s *Scheduler) killTask(
	task *Task,
	wg *sync.WaitGroup,
) {
	if wg != nil {
		defer wg.Done()
	}

	if task == nil {
		logrus.Warn("nil taskMetaData received, cannot kill task")
		return
	}

	if task.Command.CmdExecType != CmdLongRunning {
		logrus.WithField("task", task).Warn(
			"non-CmdStart task has been assigned to kill",
		)
		return
	}

	task.Lock()
	defer task.Unlock()

	if !task.isAlive() {
		return
	}

	task.signalProcess(syscall.SIGTERM)

	// wait for some time for graceful shutdown of the process
	for range sigtermWaitDuration {
		if !task.isAlive() {
			break
		}
		time.Sleep(time.Second)
	}

	if task.isAlive() {
		task.signalProcess(syscall.SIGKILL)
	}

}

// not thread safe
func (t *Task) signalProcess(signal syscall.Signal) {
	t.cmd.Process.Signal(signal)
}

// DOESN'T work on windows
// function is NOT thread safe
func (t *Task) isAlive() bool {
	// trivial case
	if t.State != StateRunning {
		return false
	}

	// task not yet launched
	if t.cmd == nil {
		return false
	}

	// command not yet started
	if t.cmd.Process == nil {
		return false
	}

	// 0 signal doesn't signal the task. It just check if the
	// process is alive and you are permitted to do so
	return t.cmd.Process.Signal(syscall.Signal(0)) == nil
}

func (s *Scheduler) queueWaitingTask(rawTask any) {
	task, ok := rawTask.(*Task)
	if !ok {
		logrus.Errorf(
			"cannot cast waiting task %v to *taskMetaData",
			rawTask,
		)
		return
	}

	s.taskQueue <- task.TaskID
}

func (t *Task) getState() TaskState {
	t.Lock()
	defer t.Unlock()

	if t.State != StateRunning {
		return t.State
	}

	if t.isAlive() {
		return StateRunning
	}

	return StateDead
}

func (s *Scheduler) getTask(taskID uuid.UUID) (*Task, error) {
	s.taskMapAndResourceLock.RLock()
	defer s.taskMapAndResourceLock.RUnlock()

	// get task
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf(
			"%w, task with id %v does not exist",
			flux_errors.ErrNotFound,
			taskID,
		)
	}

	return task, nil
}

// used to infer state of task from error returned by cmd.Wait()
func getTaskStateFromError(err error) TaskState {
	if err == nil {
		return StateCompleted
	}

	// this error is returned if process has started running
	// and then it encountered an error
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok {
			// this should never happen in linux systems
			logrus.Errorf(
				"cannot cast exit info (exitErr.Sys()): '%v' to syscall.WaitStatus",
				exitErr.Sys(),
			)
			return StateDead
		}

		// either terminated or killed
		if status.Signaled() {
			return StateKilled
		}

		// exited with non-zero exit code
		if status.ExitStatus() != 0 {
			return StateFailed
		}

		// exited with zero exit code
		return StateCompleted
	}

	if errors.Is(err, flux_errors.ErrWaitAlreadyCalled) {
		// this should never happen as we are calling Wait only once
		// if this happens then its a bug and a flaw in the design choice
		return StateDead
	}

	// command not even started. unknown error
	logrus.Errorf(
		"cannot cast error '%v' to exec.ExitError. Check if your command is correct!",
		err,
	)
	return StateFailed
}

func (t *Task) getLogger(prefix string, suffix string) *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			prefix + "id" + suffix:   t.TaskID,
			prefix + "name" + suffix: t.Name,
		},
	)
}

// not thread safe
func (s *Scheduler) cleanTaskReleasedResources() {
	for {
		select {
		case r := <-s.resourceRelease:
			s.Resources.add(r)
		case <-time.After(200 * time.Millisecond):
			return
		}
	}
}
