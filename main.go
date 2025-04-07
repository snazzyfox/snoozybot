package main

import (
	_ "snoozybot/internal/log"

	"os"
	"os/signal"

	"snoozybot/internal/bot"

	"github.com/rs/zerolog/log"
)

func main() {
	log.Info().Msg("Hello from Snoozybot!")

	botManager := bot.CreateBotManager()
	botManager.Start()
	taskManager.Start(botManager)

	// Wait for interrupt
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	log.Info().Msg("Interrupt Received. Stopping all processes.")
	taskManager.Stop()
	botManager.Stop()
}
