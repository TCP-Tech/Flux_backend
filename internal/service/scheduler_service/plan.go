package scheduler_service

import (
	"github.com/sirupsen/logrus"
)

/*
	1. Collect all running tasks with less priority than the new task
	2. Only tasks that have been executed usind CmdStart can be killed
	3. Sort the tasks based on priority and launch time (lower priority first, younger first)
	4. Iterate over the tasks and keep adding their resources until the new task can be scheduled
	5. If new task can be scheduled, kill the collected tasks and return true
	6. If new task cannot be scheduled, return false
*/

func (s *Scheduler) plan(req *Task) bool {
	// clean any released resources
	logrus.Debugf("cleaning resources before planning")
	s.cleanTaskReleasedResources()

	// premature check to avoid iterating over all tasks each time during launch
	if s.Resources.greater(req.Resources) {
		// reserve resources
		err := s.Resources.use(req.Resources)
		if err != nil {
			logrus.Error(err)
			return false
		}

		return true
	}

	taskLogger := logrus.WithFields(
		logrus.Fields{
			"req_name": req.Name,
			"req_id":   req.TaskID,
		},
	)

	taskLogger.Debugf(
		"not enough resources available for task. preparing a plan",
	)

	// get all less priority running tasks
	lowPriorityTasks := make([]*Task, 0)
	for _, task := range s.tasks {
		if task.Priority < req.Priority &&
			task.Command.CmdExecType == CmdLongRunning &&
			task.State == StateRunning {
			// Note: task is dereferenced without check
			// as nil not are sent into channel by any chance
			lowPriorityTasks = append(lowPriorityTasks, task)
		}
	}

	// tasks to kill to freeup
	tasksToKill := make([]*Task, 0)
	finalResources := s.Resources

	// sort
	sortWithPriorityAndLaunchTime(lowPriorityTasks)
	for _, task := range lowPriorityTasks {
		finalResources.add(task.Resources)
		tasksToKill = append(tasksToKill, task)
		if finalResources.greater(req.Resources) {
			break
		}
	}

	taskLogger.WithField(
		"tasksToKill", tasksToKill,
	).Debugf("plan decided to kill tasks for a request")

	if finalResources.greater(req.Resources) {
		s.killTasks(tasksToKill...)

		// clean
		logrus.Debugf("cleaning resources after killing tasks")
		s.cleanTaskReleasedResources()

		taskLogger.Debugf(
			"final resources left after preparing plan: %v",
			s.Resources,
		)

		// saftey check
		if !s.Resources.greater(req.Resources) {
			return false
		}

		// reserve resources
		err := s.Resources.use(req.Resources)
		if err != nil {
			logrus.Error(err)
			return false
		}

		return true
	}

	return false
}
