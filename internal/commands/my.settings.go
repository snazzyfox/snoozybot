package commands

import (
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"gorm.io/gorm/clause"
)

var mySettings = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "settings"},
	Subcommands: []*BotCommand{
		&mySettingsSuppressMentions,
	},
}

var mySettingsSuppressMentions = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "mentions",
		Options: []*dg.ApplicationCommandOption{
			{Name: "suppress", Type: dg.ApplicationCommandOptionBoolean, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		suppress := cd.Option("suppress").BoolValue()

		result := database.Database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"suppress_mentions"}),
		}).Create(&database.User{UserID: cd.Member.User.ID, SuppressMentions: suppress})
		if result.Error != nil {
			return result.Error
		}
		database.UserCache.Remove(cd.Member.User.ID)
		key := lo.Ternary(suppress, "my/settings/mentions.set", "my/settings/mentions.unset")
		return cd.Respond(Response{Key: key, Vars: &i18n.Vars{"suppress": suppress}})
	},
}
