package scheduler_servicetest_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/scheduler_service"
)

var scheduler scheduler_service.Scheduler

func TestMain(m *testing.M) {
	// setup
	fmt.Println("starting initializations")

	// logger
	fmt.Println("initializing logger")
	logrus.SetFormatter(&logrus.TextFormatter{
		// Force colors to be enabled
		ForceColors: true,
		// Add the full timestamp
		FullTimestamp: true,
		PadLevelText:  false,
	})
	logrus.SetLevel(logrus.DebugLevel)

	logrus.Info("initializing service")
	// no db interactions. keep database connection pool as nil
	service.InitializeServices(nil)

	logrus.Info("initializing scheduler")
	scheduler = scheduler_service.Scheduler{
		Resources: scheduler_service.Resources{
			CPU:    200,
			Memory: 2000,
		},
		QueueBuffer: 10,
	}
	scheduler.Start()

	logrus.Info("starting tests")
	code := m.Run() // runs all tests

	// teardown
	logrus.Info("tests complted")

	os.Exit(code)
}

func assertTaskState(
	t *testing.T,
	taskID uuid.UUID,
	state scheduler_service.TaskState,
	delaySeconds int32,
) {
	var currentState scheduler_service.TaskState
	for range delaySeconds {
		currentState, err := scheduler.GetTaskState(taskID)
		if err != nil {
			t.Errorf("error getting task state: %v", err)
			return
		}
		if currentState == state {
			return
		}
		t.Logf("task %v current state: %v, waiting for state: %v", taskID, currentState, state)
		time.Sleep(time.Second)
	}
	if currentState != state {
		t.Errorf("task %v did not reach state %v in %v seconds", taskID, state, delaySeconds)
	}
}

func TestTaskStart(t *testing.T) {
	var launch bool
	var complete bool

	// test start command
	startReq := scheduler_service.TaskRequest{
		Name: "start_command_test",
		Resources: scheduler_service.Resources{
			CPU:    10,
			Memory: 100,
		},
		Command: scheduler_service.Command{
			Name:        "true",
			CmdExecType: scheduler_service.CmdLongRunning,
		},
		SchedulingRetries: 1,
		OnLaunchComplete: func(res scheduler_service.TaskResponse) {
			if res.Error != nil {
				t.Errorf("command failed: %v", res.Error)
			}
			launch = true
		},
		OnTaskComplete: func(taskID uuid.UUID, err error) {
			if err != nil {
				t.Error(err)
			}
			complete = true
		},
	}

	t.Logf("scheduling start command task")
	taskID, err := scheduler.ScheduleTask(startReq)
	if err != nil {
		t.Error(err)
		return
	}

	assertTaskState(t, taskID, scheduler_service.StateCompleted, 10)

	if !launch {
		t.Errorf("start command onLaunchComplete was not called in time")
	}

	if !complete {
		t.Errorf("start command onTaskComplete was not called in time")
	}

}

func TestTaskRun(t *testing.T) {
	// test run command - returns only error
	tasks := []struct {
		name    string
		command scheduler_service.Command
		state   scheduler_service.TaskState
		err     error
	}{
		{
			name: "run_command_no_error",
			command: scheduler_service.Command{
				Name:        "sleep",
				Args:        []string{"1"},
				CmdExecType: scheduler_service.CmdRun,
			},
			state: scheduler_service.StateCompleted,
		},
		{
			name: "run_command_with_error_at_start",
			command: scheduler_service.Command{
				Name:        "non_existing_command",
				CmdExecType: scheduler_service.CmdRun,
			},
			state: scheduler_service.StateFailed,
		},
		{
			name: "run_command_with_error_at_execution",
			command: scheduler_service.Command{
				Name:        "sh",
				Args:        []string{"-c", "exit 1"},
				CmdExecType: scheduler_service.CmdRun,
			},
			state: scheduler_service.StateFailed,
		},
	}

	for _, task := range tasks {
		run := false
		req := scheduler_service.TaskRequest{
			Name: task.name,
			Resources: scheduler_service.Resources{
				CPU:    10,
				Memory: 100,
			},
			Command:           task.command,
			Priority:          50,
			SchedulingRetries: 1,
			OnLaunchComplete: func(response scheduler_service.TaskResponse) {
				if response.Error != nil {
					t.Log(response.Error)
				}
				run = true
			},
		}

		t.Logf("scheduling %v task", task.name)
		taskID, err := scheduler.ScheduleTask(req)
		if err != nil {
			t.Error(err)
			continue
		}

		assertTaskState(t, taskID, task.state, 3)

		// give time for launch function to be completed
		for range 10 {
			if run {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !run {
			t.Errorf("onLaunchComplete of task %v was not called in time", task.name)
		}
	}
}

func TestTaskOutput(t *testing.T) {
	tasks := []struct {
		name           string
		cmd            scheduler_service.Command
		expectedOutput string
		finalState     scheduler_service.TaskState
		errorMessag    string
	}{
		{
			name: "output_no_error",
			cmd: scheduler_service.Command{
				Name:        "echo",
				Args:        []string{"hello"},
				CmdExecType: scheduler_service.CmdOutput,
			},
			expectedOutput: "hello\n",
			finalState:     scheduler_service.StateCompleted,
			errorMessag:    "",
		},
		{
			name: "output_with_error",
			cmd: scheduler_service.Command{
				Name:        "sh",
				Args:        []string{"-c", "echo hello && exit 1"},
				CmdExecType: scheduler_service.CmdOutput,
			},
			expectedOutput: "hello\n",
			finalState:     scheduler_service.StateFailed,
			errorMessag:    "exit status 1",
		},
		{
			name: "error_no_output",
			cmd: scheduler_service.Command{
				Name:        "sh",
				Args:        []string{"-c", "echo hello 1>&2 && exit 1"},
				CmdExecType: scheduler_service.CmdOutput,
			},
			expectedOutput: "",
			finalState:     scheduler_service.StateFailed,
			errorMessag:    "exit status 1",
		},
		{
			name: "no_output_no_error",
			cmd: scheduler_service.Command{
				Name:        "true",
				CmdExecType: scheduler_service.CmdOutput,
			},
			expectedOutput: "",
			finalState:     scheduler_service.StateCompleted,
			errorMessag:    "",
		},
	}

	for _, task := range tasks {
		output := false
		req := scheduler_service.TaskRequest{
			Name: task.name,
			Resources: scheduler_service.Resources{
				CPU:    10,
				Memory: 100,
			},
			Command:           task.cmd,
			Priority:          50,
			SchedulingRetries: 1,
			OnLaunchComplete: func(response scheduler_service.TaskResponse) {
				if task.errorMessag != "" {
					if response.Error == nil || response.Error.Error() != task.errorMessag {
						t.Errorf("task %v expected error %v, got %v", task.name, task.errorMessag, response.Error)
					}
				}
				output = true
			},
		}

		taskID, err := scheduler.ScheduleTask(req)
		if err != nil {
			t.Errorf("failed to sumbit task %v: %v", task.name, err)
		}
		assertTaskState(t, taskID, task.finalState, 3)

		// give time for launch function to be completed
		for range 10 {
			if output {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !output {
			t.Errorf("onLaunchComplete of task %v was not called in time", task.name)
		}
	}
}

func TestHightPriorityTaskKillLowPriorityTask(t *testing.T) {
	tasks := []struct {
		name              string
		priority          int32
		schedulingRetries int32
		cmd               scheduler_service.Command
		resources         scheduler_service.Resources
		finalState        scheduler_service.TaskState
	}{
		{
			name:     "low_priority_task",
			priority: 30,
			cmd: scheduler_service.Command{
				Name:        "sleep",
				Args:        []string{"100"},
				CmdExecType: scheduler_service.CmdLongRunning,
			},
			schedulingRetries: 3,
			finalState:        scheduler_service.StateKilled,
			resources: scheduler_service.Resources{
				CPU:    199,
				Memory: 1999,
			},
		},
		{
			name:     "high_priority_task",
			priority: 50,
			cmd: scheduler_service.Command{
				Name:        "true",
				CmdExecType: scheduler_service.CmdRun,
			},
			schedulingRetries: 3,
			finalState:        scheduler_service.StateCompleted,
			resources: scheduler_service.Resources{
				CPU:    10,
				Memory: 100,
			},
		},
	}

	ids := make([]uuid.UUID, 0)

	for _, task := range tasks {
		req := scheduler_service.TaskRequest{
			Name:              task.name,
			Resources:         task.resources,
			Command:           task.cmd,
			Priority:          task.priority,
			SchedulingRetries: task.schedulingRetries,
			OnLaunchComplete: func(response scheduler_service.TaskResponse) {
				t.Logf("task %v's response: %v", task.name, response)
			},
		}

		taskID, err := scheduler.ScheduleTask(req)
		if err != nil {
			t.Errorf(
				"task %v cannot scheduled: %v",
				task.name,
				err,
			)
			return
		}
		ids = append(ids, taskID)
	}

	for idx, task := range tasks {
		assertTaskState(t, ids[idx], task.finalState, 10)
	}
}
