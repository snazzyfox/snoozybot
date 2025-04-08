package bot

import (
	"runtime/debug"
	"snoozybot/internal/commands"
	"snoozybot/internal/config"
	"snoozybot/internal/events"
	"sync"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

type BotManager struct {
	bots      []*dg.Session
	GuildBots map[string]*dg.Session
	stop      chan struct{}
	wg        sync.WaitGroup
	ready     sync.WaitGroup
}

func CreateBotManager() *BotManager {
	bm := &BotManager{bots: []*dg.Session{}, GuildBots: make(map[string]*dg.Session), stop: make(chan struct{}), wg: sync.WaitGroup{}}

	guildTokens, err := config.DiscordToken.GetValues()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to get discord tokens")
	}

	// Convert guild->token map to token->[guilds]; some guilds may share the same token/bot user
	tokenGuilds := make(map[string][]string)
	for guild, value := range guildTokens {
		tokenGuilds[value] = append(tokenGuilds[value], guild)
	}

	// Create all bots
	for token, guilds := range tokenGuilds {
		bot := createBot(token, guilds, &bm.ready)
		bm.bots = append(bm.bots, bot)
		for _, guild := range guilds {
			bm.GuildBots[guild] = bot
		}
	}

	return bm
}

// Starts all bots previously created by CreateBots
// This function will block and return after all bots have reported ready
func (bm *BotManager) Start() {
	bm.stop = make(chan struct{})
	for _, bot := range bm.bots {
		bot := bot // each goroutine needs a separate copy
		bm.wg.Add(1)
		bm.ready.Add(1)
		go func() {
			defer bm.wg.Done()

			if err := bot.Open(); err != nil {
				log.Panic().Err(err).Msg("Failed to start gateway client")
			}

			<-bm.stop // wait for signal from main to stop the bot
			log.Info().Msg("Received stop signal. Stopping bot...")

			if err := bot.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close bot session gracefully")
			}
		}()
	}
	bm.ready.Wait()
}

func (bm *BotManager) Stop() {
	close(bm.stop)
	bm.wg.Wait()
}

func createBot(token string, guilds []string, ready *sync.WaitGroup) *dg.Session {
	var logger = log.Logger // will have the bot name attached once bot starts and figures out who it is

	bot, err := dg.New("Bot " + token)
	if err != nil {
		logger.Panic().Err(err).Strs("guilds", guilds).Msg("Failed to start bot")
	}

	bot.Identify.Intents = dg.IntentsAll

	// Bot onInteraction
	bot.AddHandler(func(s *dg.Session, i *dg.InteractionCreate) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error().Err(rec.(error)).Bytes("stack", debug.Stack()).Msg("Captured panic during command handling")
			}
		}()

		switch i.Type {
		case dg.InteractionApplicationCommand, dg.InteractionApplicationCommandAutocomplete:
			data := i.ApplicationCommandData()
			name := data.Name
			cmd := *botCommands[name]
			logger.Debug().Str("type", i.Type.String()).Str("guild", i.GuildID).Str("name", name).Any("options", data.Options).Msg("Received application command")
			if err := cmd.CommandHandler(&commands.CommandData{
				Session:           s,
				InteractionCreate: i,
				Log:               logger.With().Str("interaction", i.Type.String()).Str("command", name).Str("guild", i.GuildID).Str("author", i.Member.User.Username).Logger(),
			}); err != nil {
				logger.Error().Any("interaction", i).Err(err).Msg("Error handling interaction")
			}
		default:
			logger.Warn().Any("event", i).Msg("Received unreconized interaction create event. This event has been ignored.")
		}
	})

	// Bot onReady
	bot.AddHandler(func(s *dg.Session, r *dg.Ready) {
		if app, err := s.Application("@me"); err == nil {
			logger = logger.With().Str("application", app.Name).Logger()
		} else {
			logger.Error().Err(err).Msg("Failed to fetch application info.")
		}
		logger.Info().Str("bot", r.User.Username).Msg("Bot started.")

		// Register commands on discord
		logger.Info().Msg("Registering application commands.")
		for _, guildId := range guilds {
			if _, err := bot.ApplicationCommandBulkOverwrite(r.Application.ID, guildId, commandList); err != nil {
				logger.Error().Err(err).Msg("Failed to register application commands.")
			}
		}

		logger.Info().Msg("Bot is now ready to accept commands.")
		ready.Done()
	})

	// Register application event handlers
	for _, event := range events.Events {
		bot.AddHandler(event)
	}
	return bot
}

var commandList []*dg.ApplicationCommand = lo.Map(
	commands.Commands,
	func(item *commands.BotCommand, _ int) *dg.ApplicationCommand {
		// Automatically insert all the i18n stuff, so I don't have to keep repeating them everywhere
		item.Build()
		cmd := &item.ApplicationCommand
		return cmd
	},
)

var botCommands = lo.KeyBy(commands.Commands, func(item *commands.BotCommand) string { return item.Name })
