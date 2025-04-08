package events

import (
	"runtime/debug"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type EventData[T any] struct {
	Session *dg.Session
	Event   *T
	Logger  *zerolog.Logger
}

func createEventHandler[T any](name string, handler func(ed EventData[T]) error) func(s *dg.Session, ev *T) {
	logger := log.Logger.With().Str("event", name).Logger()
	return func(s *dg.Session, ev *T) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error().Err(rec.(error)).Bytes("stack", debug.Stack()).Msg("Captured panic during event handling")
			}
		}()
		err := handler(EventData[T]{Session: s, Event: ev, Logger: &logger})
		if err != nil {
			logger.Error().Err(err).Msg("Error handling event")
		}
	}
}
