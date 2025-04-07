package commands

import (
	"errors"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"

	dg "github.com/bwmarrin/discordgo"
	"github.com/markusmobius/go-dateparser"
	"gorm.io/gorm"
)

var myBedtime = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "bedtime",
	},
	Subcommands: []*BotCommand{
		&myBedtimeSet,
		&myBedtimeGet,
		&myBedtimeClear,
	},
}

var myBedtimeSet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "set",
		Options: []*dg.ApplicationCommandOption{
			{Name: "time", Type: dg.ApplicationCommandOptionString, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		input := cd.Option("time").StringValue()
		time, err := _bedtimeParser.Parse(&_bedtimeParserConfig, input)
		if err != nil {
			return cd.Respond(Response{Key: "my.bedtime.set.invalid"})
		}
		user := database.User{UserID: cd.Interaction.Member.User.ID}
		database.Database.Select("timezone").Take(&user)
		if user.Timezone == nil {
			return cd.Respond(Response{Key: "my.bedtime.set.timezone"})
		}
		cd.Log.Info().Time("time", time.Time).Msg("Setting user bedtime")
		if res := database.Database.Model(&user).Update("bedtime", time.Time); res.Error != nil {
			return res.Error
		}
		database.UserCache.Remove(user.UserID)
		return cd.Respond(Response{Key: "my.bedtime.set.success"})
	},
}

var myBedtimeGet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "get",
	},
	CommandHandler: func(cd *CommandData) error {
		user := database.User{UserID: cd.Interaction.Member.User.ID}
		result := database.Database.Select("bedtime").Take(&user)
		if user.Bedtime != nil {
			return cd.Respond(Response{Key: "my.bedtime.get.success", Vars: &i18n.Vars{"time": user.Bedtime.String()}})
		} else if result.Error == nil || errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return cd.Respond(Response{Key: "my.bedtime.get.missing"})
		} else {
			return result.Error
		}
	},
}

var _bedtimeParser = dateparser.Parser{
	ParserTypes: []dateparser.ParserType{
		dateparser.AbsoluteTime,
	},
}
var _bedtimeParserConfig = dateparser.Configuration{
	ReturnTimeAsPeriod: true,
}

var myBedtimeClear = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "clear",
	},
	CommandHandler: func(cd *CommandData) error {
		user := database.User{UserID: cd.Interaction.Member.User.ID}
		if res := database.Database.Model(&user).Select("bedtime").Update("timezone", nil); res.Error != nil {
			return res.Error
		}
		cd.Log.Info().Msg("Cleared user bedtime")
		database.UserCache.Remove(user.UserID)
		return cd.Respond(Response{Key: "my.bedtime.clear.success"})
	},
}
