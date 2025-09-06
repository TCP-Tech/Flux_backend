package scheduler_service

import (
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func (s *Scheduler) Start() {
	logrus.Info("initializing scheduler's taskQueue channel with buffer size ", s.QueueBuffer)
	s.taskQueue = make(chan *Task, s.QueueBuffer)

	logrus.Info("initializing scheduler's tasks map")
	s.tasks = make(map[uuid.UUID]*Task)

	logrus.Info("initializing scheduler's taskMapLock")
	s.taskMapLock = sync.RWMutex{}

	logrus.Info("initliazing channel for task released resources")
	// sending should be non-blocking in any case. so initialize with large size
	s.resourceRelease = make(chan Resources, 500)

	logrus.Info("starting a 'launch' goroutine")
	go s.launch()
}
