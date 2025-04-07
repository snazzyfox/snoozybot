package commands

import (
	"snoozybot/internal/config"
	"snoozybot/internal/database"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

var myBirthday = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "birthday",
	},
	Subcommands: []*BotCommand{
		&myBirthdaySet,
		&myBirthdayClear,
	},
}

var myBirthdaySet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "set",
		Options: []*dg.ApplicationCommandOption{
			{Name: "month", Type: dg.ApplicationCommandOptionInteger, Required: true, Choices: []*dg.ApplicationCommandOptionChoice{
				{Name: "January", Value: 1},
				{Name: "February", Value: 2},
				{Name: "March", Value: 3},
				{Name: "April", Value: 4},
				{Name: "May", Value: 5},
				{Name: "June", Value: 6},
				{Name: "July", Value: 7},
				{Name: "August", Value: 8},
				{Name: "September", Value: 9},
				{Name: "October", Value: 10},
				{Name: "November", Value: 11},
				{Name: "December", Value: 12},
			}},
			{Name: "day", Type: dg.ApplicationCommandOptionInteger, Required: true, MinValue: lo.ToPtr[float64](1), MaxValue: 31},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		// check server has a birthday channel
		birthdayChannel, err := config.ProfileBirthdayChannel.Get(cd.Interaction.GuildID).Value()
		if err != nil || birthdayChannel == "" {
			return cd.Respond(Response{Key: "my/birthday/set.no_channel"})
		}

		// check user has timezone set
		user := database.User{UserID: cd.Interaction.Member.User.ID}
		if database.Database.Select("timezone").Take(&user); user.Timezone == nil {
			return cd.Respond(Response{Key: "my/birthday/set.timezone"})
		}
		loc, err := time.LoadLocation(*user.Timezone)
		if err != nil {
			return cd.Respond(Response{Key: "my/birthday/set.timezone"})
		}

		month := cd.Option("month").IntValue()
		day := int(cd.Option("day").IntValue())

		// check date is valid using a fixed leap year
		d := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, loc)
		if d.Month() != time.Month(month) || d.Day() != day {
			return cd.Respond(Response{Key: "my/birthday/set.invalid"})
		}

		// compute the next birthday
		nextBirthday := time.Date(time.Now().Year(), time.Month(month), day, 0, 0, 0, 0, loc)
		if time.Now().After(nextBirthday) {
			nextBirthday = nextBirthday.AddDate(1, 0, 0)
		}

		if err != nil {
			return err
		}

		// delete any existing scheduled task
		cd.Log.Debug().Msg("Deleting existing birthday task")
		database.Database.Where(&database.ScheduledTask{
			GuildID: cd.Interaction.GuildID, TaskType: database.TaskTypeBirthday, UserID: cd.Interaction.Member.User.ID,
		}).Delete(&database.ScheduledTask{})

		// create the scheduled task
		if res := database.Database.Create(&database.ScheduledTask{
			GuildID:      cd.Interaction.GuildID,
			TaskType:     database.TaskTypeBirthday,
			ProcessAfter: nextBirthday,
			UserID:       cd.Interaction.Member.User.ID,
		}); res.Error != nil {
			return res.Error
		}

		cd.Log.Info().Msg("Created birthday task")
		return cd.Respond(Response{Key: "my/birthday/set.success"})
	},
}

var myBirthdayClear = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "clear",
	},
	CommandHandler: func(cd *CommandData) error {
		database.Database.Where(&database.ScheduledTask{
			GuildID: cd.Interaction.GuildID, TaskType: database.TaskTypeBirthday, UserID: cd.Interaction.Member.User.ID,
		}).Delete(&database.ScheduledTask{})
		cd.Log.Info().Msg("Cleared birthday task")
		return cd.Respond(Response{Key: "my/birthday/clear.success"})
	},
}
