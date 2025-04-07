package commands

import dg "github.com/bwmarrin/discordgo"

var my = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "my"},
	Subcommands: []*BotCommand{
		&myBedtime,
		&myTimezone,
		&myBirthday,
		&mySettings,
	},
}
