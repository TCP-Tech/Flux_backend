package submission_service

import (
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/scheduler_service"
)

// initialize the following:
//  1. Postman
//  2. NyxMaster
//  3. SubmissionQuerier
//  4. NyxManager
func (sub *SubmissionService) Start(
	scrStCmd NyxScrStrtCmd,
	cfQueryUrl string,
	dbSubPollSeconds int64,
	scheduler *scheduler_service.Scheduler,
) {
	// initlialize postman
	postman := postman{}
	postman.start()

	sub.Postman = &postman

	// NOTE: bot is for temporary usage only
	bots := make([]Bot, 0)
	bots = append(bots, Bot{
		Name: "sagarguptaa35",
		Cookies: map[string]string{
			"JSESSIONID":  "E3923F495FAFCD388A64D69397C24535",
			"pow":         "600303f5d9a73af06c0e4e55cd1415d9a8e72f20",
			"X-User":      "5d24133a28b609dd16bba0cf698ef6f0402036efd0e51929fd0be2d6e9cc6b35105d6b0c0dd427ea",
			"X-User-Sha1": "c78b37ed165923cb7ceb6053e46518742f2f45da",
		},
		Platform: platformCodeforces,
	})

	// initialize submission querier
	subQuerier := subStatManagerImpl{
		problemServiceConfig: sub.ProblemService,
		contestServiceConfig: sub.ContestService,
		db:                   sub.DB,
	}
	subQuerier.start()

	// initialize nyxMaster
	nyxMaster := nyxMaster{
		postman:   &postman,
		scrStCmd:  scrStCmd,
		bots:      bots,
		scheduler: scheduler,
	}
	nyxMaster.Start(sub.DB, cfQueryUrl, &subQuerier)

	// register with postman
	postman.RegisterMailClient(mailNyxMaster, &nyxMaster)

	// initialize nyx manager
	nyxManager := nyxManager{
		db:            sub.DB,
		postman:       &postman,
		subQrr:        &subQuerier,
		probSerConfig: sub.ProblemService,
	}
	nyxManager.Start(dbSubPollSeconds)

	// register manager with postman
	postman.RegisterMailClient(mailNyxManager, &nyxManager)

	sub.EvaluatorMails = map[string]Evaluator{
		platformCodeforces: &nyxManager,
	}

	logrus.Info("initialized submission service")
}
