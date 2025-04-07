package commands

import (
	"snoozybot/internal/config"
	"snoozybot/internal/i18n"
	"strings"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

var report = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "report",
		Options: []*dg.ApplicationCommandOption{
			{Name: "user", Type: dg.ApplicationCommandOptionUser, Required: true},
			{Name: "location", Type: dg.ApplicationCommandOptionString, Required: true, Choices: []*dg.ApplicationCommandOptionChoice{
				{Value: "Private Message"},
				{Value: "This Server"},
				{Value: "Another Server"},
				{Value: "Outside Discord"},
			}},
			{Name: "screenshot", Type: dg.ApplicationCommandOptionAttachment, Required: true},
			{Name: "comment", Type: dg.ApplicationCommandOptionString, Required: false},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		channel, err := config.ReportChannelId.Get(cd.GuildID).Value()
		if err != nil {
			return cd.Respond(Response{Key: "report.notAvailable"})
		}
		screenshotAttachmentId := cd.Option("screenshot").Value.(string)
		screenshot := cd.ApplicationCommandData().Resolved.Attachments[screenshotAttachmentId]
		if screenshot == nil || !strings.HasPrefix(screenshot.ContentType, "image/") {
			return cd.Respond(Response{Key: "report.invalidImage"})
		}
		// Localization after this part is based on the server settings, not the user sending the message

		cd.ChannelMessageSendComplex(string(channel), &dg.MessageSend{
			Content: lo.CoalesceOrEmpty(cd.Option("comment").StringValue()),
			Embeds: []*dg.MessageEmbed{{
				Title: i18n.Get(*cd.GuildLocale, "report.title"),
				Fields: []*dg.MessageEmbedField{
					{Name: i18n.Get(*cd.GuildLocale, "report.originator"), Value: cd.Member.Mention()},
					{Name: i18n.Get(*cd.GuildLocale, "report.location"), Value: cd.Option("location").StringValue()},
					{Name: i18n.Get(*cd.GuildLocale, "report.target"), Value: cd.Option("user").UserValue(cd.Session).Mention()},
				},
				Image: &dg.MessageEmbedImage{
					URL:      screenshot.URL,
					ProxyURL: screenshot.ProxyURL,
					Width:    screenshot.Width,
					Height:   screenshot.Height,
				},
			}},
		})
		// Respond to the user
		cd.Respond(Response{Key: "report.success"})
		return nil
	},
}
