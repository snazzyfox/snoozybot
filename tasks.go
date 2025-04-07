package main

import (
	"context"
	"runtime/debug"
	"snoozybot/internal/bot"
	"snoozybot/internal/tasks"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type TaskManager struct {
	tasks      []*tasks.PeriodicTask
	Ready      sync.WaitGroup
	stop       chan struct{}
	botManager *bot.BotManager
	wg         sync.WaitGroup
}

var taskManager = &TaskManager{
	tasks: tasks.Tasks,
	stop:  make(chan struct{}),
}

var globalCtx, cancelGlobalCtx = context.WithCancel(context.Background())

func (tm *TaskManager) Start(botManager *bot.BotManager) {
	tm.botManager = botManager

	log.Info().Msg("Waiting for all bots to be ready before starting tasks...")
	tm.Ready.Wait()

	log.Info().Msg("All bots ready, starting tasks...")
	for _, task := range tm.tasks {
		log.Info().Str("task", task.Name).Msg("Starting periodic task")
		ticker := time.NewTicker(task.Interval)
		tm.wg.Add(1)
		go func(task *tasks.PeriodicTask, ticker *time.Ticker) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error().Str("task", task.Name).Err(rec.(error)).Bytes("stack", debug.Stack()).Msg("Panic captured during periodic task")
				}
				ticker.Stop()
				log.Info().Str("task", task.Name).Msg("Stopped periodic task")
				tm.wg.Done()
			}()
			exec := func() {
				td := &tasks.TaskData{
					BotManager: tm.botManager,
					Logger:     log.With().Str("task", task.Name).Logger(),
					Context:    context.WithoutCancel(globalCtx),
				}
				if err := task.TaskHandler(td); err != nil {
					log.Error().Err(err).Msg("Error received during periodic task")
				}
			}

			exec() // ticker runs at the end of the period
			for {
				select {
				case <-tm.stop:
					return
				case <-ticker.C:
					exec()
				}
			}
		}(task, ticker)
	}
}

func (tm *TaskManager) Stop() {
	log.Info().Msg("Received stop signal. Stopping all tasks...")
	cancelGlobalCtx()
	close(tm.stop)
	tm.wg.Wait()
}
