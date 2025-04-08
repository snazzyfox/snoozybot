package commands

import (
	"snoozybot/internal/config"

	dg "github.com/bwmarrin/discordgo"
)

var admin = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "admin", DefaultMemberPermissions: &CommandPermissionAdminOnly},
	Subcommands: []*BotCommand{
		&adminConfig,
	},
}

var adminConfig = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "config"},
	Subcommands: []*BotCommand{
		&adminConfigReload,
	},
}

var adminConfigReload = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "reload"},
	CommandHandler: func(cd *CommandData) error {
		cd.Log.Info().Str("requestedInGuild", cd.GuildID).Str("requestedBy", cd.Member.User.ID).Msg("Reloading all configs")
		config.ClearCache()
		return cd.Respond(Response{Key: "admin.config.reload.success"})
	},
}
