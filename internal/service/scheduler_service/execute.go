package scheduler_service

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (s *Scheduler) execute(task *Task) {
	// long running task
	if task.Command.CmdExecType == CmdLongRunning {
		s.executeLongRunningTask(task)
		return
	}

	// all short-running tasks will never go to StateRunning as per current plan
	task.Lock()
	defer task.Unlock()

	executeLogger := task.getLogger("executable_", "")

	executeLogger.Info("executing task")

	var out []byte
	var err error

	// form the cmd object
	// Note: cmd is a pointer to exec.Cmd. exec.Command() never returns nil
	cmd := exec.Command(task.Command.Name, task.Command.Args...)

	// This hook is called right before execve() in the child process.
	// We use syscall.SysProcAttr to set up the child's environment.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig is the specific Linux option that tells the child:
		// "If my parent dies, send me this signal."
		Pdeathsig: syscall.SIGKILL,
	}

	switch task.Command.CmdExecType {
	case CmdRun:
		err = cmd.Run()
	case CmdOutput:
		out, err = cmd.Output()
	case CmdCombined:
		out, err = cmd.CombinedOutput()
	default:
		err = fmt.Errorf(
			"%w, unknown command execution type: %v",
			flux_errors.ErrInvalidRequest,
			task.Command.CmdExecType,
		)
	}

	s.resourceRelease <- task.Resources

	task.cmd = cmd
	task.State = getTaskStateFromError(err)

	// inform
	response := TaskResponse{
		TaskID: task.TaskID,
		Out:    out,
		Error:  err,
	}

	executeLogger.Debug("launching a gor calling OnLaunchComplete")
	go task.OnLaunchComplete(response)
}

func (s *Scheduler) executeLongRunningTask(task *Task) {
	task.Lock()
	defer task.Unlock()

	executeLogger := task.getLogger("executable_", "")

	executeLogger.Info("executing task")

	cmd := exec.Command(task.Command.Name, task.Command.Args...)

	// This hook is called right before execve() in the child process.
	// We use syscall.SysProcAttr to set up the child's environment.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig is the specific Linux option that tells the child:
		// "If my parent dies, send me this signal."
		Pdeathsig: syscall.SIGKILL,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		go task.OnLaunchComplete(
			TaskResponse{
				task.TaskID,
				nil,
				err,
			},
		)
		// cancel the context as the command failed to start
		return
	}

	task.State = StateRunning
	task.cmd = cmd

	executeLogger.Debugf("process id: %v", cmd.Process.Pid)

	executeLogger.Debug("launching a gor to monitor task for complete")
	go s.waitForTaskComplete(task)

	executeLogger.Debug("launching a gor calling OnLaunchComplete")
	go task.OnLaunchComplete(
		TaskResponse{
			task.TaskID,
			nil,
			nil,
		},
	)
}

func (s *Scheduler) waitForTaskComplete(task *Task) {
	err := task.cmd.Wait()

	// release the task's resources
	s.resourceRelease <- task.Resources

	// prepare a cutom logger
	deadLogger := task.getLogger("dead_task_", "")
	deadLogger.Warn("task is dead")

	task.Lock()
	defer task.Unlock()

	// update its state
	task.State = getTaskStateFromError(err)

	// inform
	if task.OnTaskComplete != nil {
		deadLogger.Debugf("launching a gor calling onTaskComplete")
		go task.OnTaskComplete(task.TaskID, err)
	}
}
