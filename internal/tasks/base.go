package tasks

import (
	"context"
	"snoozybot/internal/bot"
	"time"

	"github.com/rs/zerolog"
)

type PeriodicTask struct {
	Name        string
	Interval    time.Duration
	TaskHandler func(ctx *TaskData) error
}

type TaskData struct {
	BotManager *bot.BotManager
	Logger     zerolog.Logger
	Context    context.Context
}
