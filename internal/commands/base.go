package commands

import (
	"fmt"
	"snoozybot/internal/i18n"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

var CommandPermissionAdminOnly = int64(0)
var CommandPermissionModeratorOnly = int64(dg.PermissionManageMessages)

type CommandData struct {
	*dg.Session
	*dg.InteractionCreate
	Log zerolog.Logger
}

type Response struct {
	dg.InteractionResponseData
	Key    string
	Vars   *i18n.Vars
	Public bool
}

func (cd *CommandData) Option(name string) *dg.ApplicationCommandInteractionDataOption {
	// Not converting option values to a map here. Option list is short and the overhead
	// of conversion is usually worse than just looping through the array a couple times.
	for _, opt := range cd.InteractionCreate.ApplicationCommandData().Options {
		if opt.Name == name {
			return opt
		}
	}
	return nil
}

func (cd *CommandData) Respond(r Response) error {
	if r.Key != "" {
		locale := lo.Ternary(r.Flags&dg.MessageFlagsEphemeral != 0, cd.Locale, *cd.GuildLocale)
		r.Content = i18n.Get(locale, r.Key, r.Vars)
	}
	if r.Public {
		r.Flags &= ^dg.MessageFlagsEphemeral
	} else {
		r.Flags |= dg.MessageFlagsEphemeral
	}
	return cd.InteractionRespond(cd.Interaction, &dg.InteractionResponse{
		Type: dg.InteractionResponseChannelMessageWithSource,
		Data: &r.InteractionResponseData,
	})
}

type CommandHandler func(*CommandData) error

type BotCommand struct {
	dg.ApplicationCommand
	Subcommands    []*BotCommand
	CommandHandler CommandHandler
	subcommandMap  map[string]*BotCommand
}

/* Builds the subcommand tree. The command will be missing data for discord before Build() is called. */
func (bc *BotCommand) Build() *BotCommand {
	return bc.build("")
}

func (bc *BotCommand) hasSubcommands() bool {
	return len(bc.Subcommands) > 0
}

func (bc *BotCommand) build(prefix string) *BotCommand {
	localizationKey := lo.Ternary(prefix == "", bc.Name, prefix+"/"+bc.Name)
	if bc.hasSubcommands() {
		if len(bc.Options) > 0 || bc.CommandHandler != nil {
			log.Panic().Str("command", bc.Name).Msg("Cannot determine if this is a subcommand. If subcommands are defined, all handlers and options must be left unset.")
		}
		log.Debug().Str("command", bc.Name).Msg("Generating subcommands")
		bc.subcommandMap = make(map[string]*BotCommand)
		for _, sc := range bc.Subcommands {
			bc.subcommandMap[sc.Name] = sc.build(localizationKey)
			bc.Options = append(bc.Options, sc.asSubcommandOption())
		}
		// Write the command handler if it's a subcommand
		bc.CommandHandler = func(cd *CommandData) error {
			data := cd.ApplicationCommandData()
			scName := data.Options[0].Name
			if subcommand, ok := bc.subcommandMap[scName]; ok {
				// Peel off outer layer of options data
				data.Options = data.Options[0].Options
				cd.InteractionCreate.Data = data
				return subcommand.CommandHandler(cd)
			} else {
				return fmt.Errorf("subcommand %s does not exist for parent command %s", scName, bc.Name)
			}
		}
	}
	bc.localizeCommand(localizationKey)
	return bc
}

func (bc *BotCommand) localizeCommand(key string) {
	// Add localization
	log.Trace().Str("command", bc.Name).Msg("Localizing command")
	bc.NameLocalizations = i18n.GetMetadata(key + ".name")
	if bc.Type != dg.UserApplicationCommand && bc.Type != dg.MessageApplicationCommand {
		if bc.hasSubcommands() {
			bc.Description = "(unused)" // it'll be missing
		} else {
			descKey := key + ".description"
			bc.DescriptionLocalizations = i18n.GetMetadata(descKey)
			bc.Description = (*bc.DescriptionLocalizations)[dg.EnglishUS]
		}
	}
	if !bc.hasSubcommands() {
		// subcommands descriptions are just a placeholder, they're unused
		for _, opt := range bc.Options {
			opt.NameLocalizations = *i18n.GetMetadata(key + ".options." + opt.Name + ".name")
			opt.DescriptionLocalizations = *i18n.GetMetadata(key + ".options." + opt.Name + ".description")
			opt.Description = opt.DescriptionLocalizations[dg.EnglishUS]
			for _, cho := range opt.Choices {
				cho.NameLocalizations = *i18n.GetMetadata(key + ".options." + opt.Name + ".choices." + fmt.Sprint(cho.Value))
				cho.Name = cho.NameLocalizations[dg.EnglishUS]
			}
		}
	}
}

/* Returns the BotCommand's data, but encoded as a subcommand instead. Only works after calling calling build. */
func (bc *BotCommand) asSubcommandOption() *dg.ApplicationCommandOption {
	return &dg.ApplicationCommandOption{
		Name:                     bc.Name,
		NameLocalizations:        lo.FromPtrOr(bc.NameLocalizations, map[dg.Locale]string{}),
		Description:              bc.Description,
		DescriptionLocalizations: lo.FromPtrOr(bc.DescriptionLocalizations, map[dg.Locale]string{}),
		Options:                  bc.Options,
		Type:                     lo.Ternary(len(bc.Subcommands) > 0, dg.ApplicationCommandOptionSubCommandGroup, dg.ApplicationCommandOptionSubCommand),
	}
}
