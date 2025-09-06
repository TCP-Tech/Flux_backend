package scheduler_service

import (
	"github.com/google/uuid"
)

func (s *Scheduler) GetTaskState(taskID uuid.UUID) (TaskState, error) {
	task, err := s.getTask(taskID)
	if err != nil {
		return StateUnknown, err
	}

	return task.getState(), nil
}

func (s *Scheduler) KillTask(taskID uuid.UUID) error {
	task, err := s.getTask(taskID)
	if err != nil {
		return err
	}

	state := task.getState()
	if state != StateRunning {
		// don't return error as caller might see state as running
		// and then call this function. In-between state might have been changed
		return nil
	}

	s.killTasks(task)

	return nil
}
