package scheduler_service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (s *Scheduler) ScheduleTask(req TaskRequest) (uuid.UUID, error) {
	// validate first
	err := validateTaskRequest(req)
	if err != nil {
		return uuid.Nil, err
	}

	// generate a random taskID
	taskID := uuid.New()

	task := Task{
		TaskRequest:     req,
		TaskID:          taskID,
		State:           StateQueued,
		errorChan:       make(chan error),
		QueueTime:       time.Now(),
		SchedulingTries: 0,
	}

	logrus.WithFields(
		logrus.Fields{
			"task_id":   task.TaskID,
			"task_name": task.Name,
		},
	).Info("queueing task")

	s.taskMapLock.Lock()
	defer s.taskMapLock.Unlock()

	s.tasks[taskID] = &task
	s.taskQueue <- &task

	return taskID, nil
}

func validateTaskRequest(req TaskRequest) error {
	zeroResources := Resources{}

	if zeroResources.greater(req.Resources) {
		return fmt.Errorf(
			"%w, requested resources cannot be negative",
			flux_errors.ErrInvalidRequest,
		)
	}

	if req.Priority < 0 {
		return fmt.Errorf(
			"%w, task priority cannot be negative",
			flux_errors.ErrInvalidRequest,
		)
	}

	if req.OnLaunchComplete == nil {
		return fmt.Errorf(
			"%w, OnLaunchComplete callback cannot be nil",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}
