package scheduler_service

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (s *Scheduler) launch() {
	for req := range s.taskQueue {
		s.launchTask(req)
	}
}

func (s *Scheduler) launchTask(req *Task) {
	if req == nil {
		logrus.Error("nil taskMetaData received, cannot launch task")
		return
	}

	// aquire the lock for planning and cleaning purposes
	s.taskMapLock.Lock()
	defer s.taskMapLock.Unlock()

	// get task's data from map
	task, ok := s.tasks[req.TaskID]
	if !ok {
		err := fmt.Errorf(
			"%w, task with id %v is absent in the map while launching",
			flux_errors.ErrTaskLaunchError,
			req.TaskID,
		)
		logrus.Error(err)
		return
	}

	// aquire the task's
	task.Lock()
	defer task.Unlock()

	// get logger
	launchLogger := task.getLogger("launchable_", "")

	// increase its scheduling tries
	task.SchedulingTries++

	// plan
	canLaunch := s.plan(req)
	if !canLaunch {
		// max retries exceeded, cannot launch the task as resources are too busy
		if task.SchedulingTries >= task.SchedulingRetries {
			err := fmt.Errorf(
				"%w, cannot prepare a plan after %v tries",
				flux_errors.ErrTaskLaunchError,
				task.SchedulingRetries,
			)
			launchLogger.Error(err)
			res := TaskResponse{
				TaskID: req.TaskID,
				Out:    nil,
				Error:  err,
			}
			task.OnLaunchComplete(res)
			return
		}

		// log
		launchLogger.Infof(
			"task with priority %v cannot be launched on try %v. sending it to sleep",
			task.Priority,
			task.SchedulingTries,
		)

		// increase its priority to avoid unfairness
		(*req).Priority += 10

		// send it to sleep
		service.AddToWaitQueue(
			service.WaitElement{
				Element: req,
				Process: s.queueWaitingTask,
				DelayMS: 5000,
			},
		)
		return
	}

	// update the task
	req.LaunchTime = time.Now()

	// execute the request
	go s.execute(req)
}
