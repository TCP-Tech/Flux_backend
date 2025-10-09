package submission_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/scheduler_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
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
	emailService *email.EmailService,
) {
	// validate fields
	for _, field := range []struct {
		field any
		name  string
	}{
		{sub.ProblemService, "problem service"}, {sub.ContestService, "contest service"},
		{sub.UserService, "user service"}, {sub.DB, "database"},
	} {
		if field.field == nil {
			panic(fmt.Sprintf("submission service expects non-nil %v", field.name))
		}
	}

	sub.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailSubmissionService,
		},
	)

	// initlialize postman
	postman := postman{}
	postman.start()

	sub.Postman = &postman

	// NOTE: bot is for temporary usage only
	// bots := make([]Bot, 0)
	// bots = append(bots, Bot{
	// 	Name: "sagarguptaa35",
	// 	Cookies: map[string]string{
	// 		"JSESSIONID":  "E3923F495FAFCD388A64D69397C24535",
	// 		"pow":         "600303f5d9a73af06c0e4e55cd1415d9a8e72f20",
	// 		"X-User":      "5d24133a28b609dd16bba0cf698ef6f0402036efd0e51929fd0be2d6e9cc6b35105d6b0c0dd427ea",
	// 		"X-User-Sha1": "c78b37ed165923cb7ceb6053e46518742f2f45da",
	// 	},
	// 	Platform: platformCodeforces,
	// })

	// initialize submission querier
	subQuerier := subStatManagerImpl{
		problemServiceConfig: sub.ProblemService,
		contestServiceConfig: sub.ContestService,
		db:                   sub.DB,
	}
	subQuerier.start()

	// initialize nyxMaster
	nyxMaster := nyxMaster{
		postman:      &postman,
		scrStCmd:     scrStCmd,
		scheduler:    scheduler,
		db:           sub.DB,
		emailService: emailService,
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

func (sub *SubmissionService) AddBot(ctx context.Context, bot Bot) (Bot, error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Bot{}, err
	}

	// authorize
	if err = sub.UserService.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried for manager access to add a bot", claims.UserName),
	); err != nil {
		return Bot{}, err
	}

	// create a new transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot get a transaction to add a bot %v to db: %w",
			flux_errors.ErrInternal,
			bot.Name,
			err,
		)
		return Bot{}, err
	}
	defer tx.Rollback(ctx)

	qtx := sub.DB.WithTx(tx)

	// marshal cookies
	cookieBytes, err := json.Marshal(bot.Cookies)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal cookies: %w",
			flux_errors.ErrInvalidRequest,
			err,
		)
		sub.logger.Error(err)
		return Bot{}, err
	}

	// insert bot to db
	dbBot, err := qtx.InsertBot(ctx, database.InsertBotParams{
		Name:     bot.Name,
		Platform: bot.Platform,
		Cookies:  json.RawMessage(cookieBytes),
	})
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot insert bot %v into db", bot.Name),
		)
		return Bot{}, err
	}

	fluxBot, err := dbBotToFluxBot(dbBot)
	if err != nil {
		return Bot{}, err
	}

	// refresh bots
	if err = sub.RefreshBots(ctx); err != nil {
		sub.logger.Error(
			"cannot refresh bots after adding bot to db. reverting transaction",
		)
		return Bot{}, err
	}

	//commit transaction
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after adding bot %v, %w",
			flux_errors.ErrInternal,
			bot.Name,
			err,
		)
		return Bot{}, err
	}

	return fluxBot, nil
}

func (sub *SubmissionService) RefreshBots(ctx context.Context) error {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// authroize
	if err = sub.UserService.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf(
			"user %s tried to for manager access to refresh bots",
			claims.UserName,
		),
	); err != nil {
		return err
	}

	sub.Postman.postMail(mail{
		from:     mailSubmissionService,
		to:       mailNyxMaster,
		body:     refreshBots{},
		priority: prNyxMstRefreshBots,
	})

	sub.logger.Info("requested master to refresh bots")

	return nil
}

func (sub *SubmissionService) UpdateBot(ctx context.Context, bot Bot) (Bot, error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Bot{}, err
	}

	// authorize
	if err = sub.UserService.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried for manager access to update a bot", claims.UserName),
	); err != nil {
		return Bot{}, err
	}

	// create a new transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot get a transaction to update bot %v in db: %w",
			flux_errors.ErrInternal,
			bot.Name,
			err,
		)
		return Bot{}, err
	}
	defer tx.Rollback(ctx)

	qtx := sub.DB.WithTx(tx)

	// marshal cookies
	cookieBytes, err := json.Marshal(bot.Cookies)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal cookies: %w",
			flux_errors.ErrInvalidRequest,
			err,
		)
		sub.logger.Error(err)
		return Bot{}, err
	}

	// update bot in db
	dbBot, err := qtx.UpdateBot(ctx, database.UpdateBotParams{
		Name:    bot.Name,
		Cookies: json.RawMessage(cookieBytes),
	})
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot update bot %v in db", bot.Name),
		)
		return Bot{}, err
	}

	fluxBot, err := dbBotToFluxBot(dbBot)
	if err != nil {
		return Bot{}, err
	}

	// refresh bots
	if err = sub.RefreshBots(ctx); err != nil {
		sub.logger.Error(
			"cannot refresh bots after updating bot in db. reverting transaction",
		)
		return Bot{}, err
	}

	// commit transaction
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after updating bot %v, %w",
			flux_errors.ErrInternal,
			bot.Name,
			err,
		)
		return Bot{}, err
	}

	return fluxBot, nil
}

func (sub *SubmissionService) DeleteBot(ctx context.Context, name string) error {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// authorize
	if err = sub.UserService.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried for manager access to delete a bot", claims.UserName),
	); err != nil {
		return err
	}

	// delete bot
	err = sub.DB.DeleteBot(ctx, name)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot delete bot %v", name),
		)
		return err
	}

	// refresh bots
	if err = sub.RefreshBots(ctx); err != nil {
		sub.logger.Errorf(
			"cannot refresh bots after deleting bot %v from db. reverting transaction",
			name,
		)
	}

	return nil
}

func (sub *SubmissionService) GetBots(ctx context.Context) ([]Bot, error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// authorize
	if err = sub.UserService.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried for manager access to see bots", claims.UserName),
	); err != nil {
		return nil, err
	}

	// get bots from db
	dbBots, err := sub.DB.GetBots(ctx)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			"cannot get bots from db",
		)
		return nil, err
	}

	// convert bots
	bots := make([]Bot, 0)
	for _, dbBot := range dbBots {
		bot, err := dbBotToFluxBot(dbBot)
		if err != nil {
			logrus.Errorf("cannot convert db bot %v to flux bot. cannot process request GetBots")
			return nil, err
		}

		bots = append(bots, bot)
	}

	return bots, nil
}
