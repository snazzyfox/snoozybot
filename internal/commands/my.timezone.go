package commands

import (
	"errors"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"snoozybot/internal/timezones"
	"strings"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var myTimezone = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "timezone",
	},
	Subcommands: []*BotCommand{
		&myTimezoneSet,
		&myTimezoneGet,
	},
}

var myTimezoneGet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "get",
	},
	CommandHandler: func(cd *CommandData) error {
		user := database.User{UserID: cd.Interaction.Member.User.ID}
		result := database.Database.Select("timezone").Take(&user)
		if user.Timezone != nil {
			return cd.Respond(Response{Key: "my/timezone/get.success", Vars: &i18n.Vars{"zone": user.Timezone}})
		} else if result.Error == nil || errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return cd.Respond(Response{Key: "my/timezone/get.missing"})
		} else {
			return result.Error
		}
	},
}

var myTimezoneSet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "set",
		Options: []*dg.ApplicationCommandOption{
			{Name: "zone", Type: dg.ApplicationCommandOptionString, Required: true, Autocomplete: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		if cd.Type == dg.InteractionApplicationCommandAutocomplete {
			option := cd.Option("zone")
			if option == nil || !option.Focused {
				return nil
			}
			value := strings.ToLower(option.StringValue())
			var found []*dg.ApplicationCommandOptionChoice
			for _, timezone := range timezones.TimeZones {
				if strings.Contains(strings.ToLower(timezone), value) {
					found = append(found, &dg.ApplicationCommandOptionChoice{Name: timezone, Value: timezone})
				}
				if len(found) >= 10 {
					break
				}
			}
			return cd.InteractionRespond(cd.Interaction, &dg.InteractionResponse{
				Type: dg.InteractionApplicationCommandAutocompleteResult,
				Data: &dg.InteractionResponseData{Choices: found},
			})
		} else {
			zoneName := cd.Option("zone").StringValue()
			location, err := time.LoadLocation(zoneName)
			normalized := location.String()
			if err != nil {
				return cd.Respond(Response{Key: "my/timezone/set.invalid", Vars: &i18n.Vars{"zone": zoneName}})
			}
			result := database.Database.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"timezone"}),
			}).Create(&database.User{UserID: cd.Member.User.ID, Timezone: &normalized})
			if result.Error != nil {
				return result.Error
			}
			database.UserCache.Remove(cd.Member.User.ID)
			cd.Log.Info().Str("zone", normalized).Msg("Set user timezone")
			return cd.Respond(Response{Key: "my/timezone/set.success", Vars: &i18n.Vars{"zone": normalized}})
		}
	},
}
