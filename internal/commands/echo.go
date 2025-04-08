package commands

import (
	dg "github.com/bwmarrin/discordgo"
)

var echo = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name:                     "echo",
		DefaultMemberPermissions: &CommandPermissionAdminOnly,
		Options: []*dg.ApplicationCommandOption{
			{Name: "text", Type: dg.ApplicationCommandOptionString, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		return cd.Respond(Response{InteractionResponseData: dg.InteractionResponseData{Content: cd.Option("text").StringValue()}})
	},
}
