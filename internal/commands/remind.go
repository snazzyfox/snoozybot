package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/markusmobius/go-dateparser"
	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var reminder = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "reminder",
	},
	Subcommands: []*BotCommand{
		&reminderSet,
		&reminderCancel,
		&reminderList,
	},
}

var _remindTimeParser = dateparser.Parser{}

var reminderSet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "set",
		Options: []*dg.ApplicationCommandOption{
			{Name: "time", Type: dg.ApplicationCommandOptionString, Required: true},
			{Name: "message", Type: dg.ApplicationCommandOptionString, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		user := database.User{UserID: cd.Member.User.ID}
		res := database.Database.Select("timezone").Take(&user)
		if user.Timezone == nil {
			return cd.Respond(Response{Key: "reminder/set.timezone"})
		} else if res.Error != nil {
			return res.Error
		}
		timeStr := cd.Option("time").StringValue()
		message := cd.Option("message").StringValue()
		timezone := lo.Must(time.LoadLocation(*user.Timezone))
		parsedTime, err := _remindTimeParser.Parse(&dateparser.Configuration{
			PreferredDateSource: dateparser.Future,
			DefaultTimezone:     timezone,
		}, timeStr)
		if err != nil {
			return cd.Respond(Response{Key: "reminder/set.format"})
		}
		if parsedTime.Time.Before(time.Now()) {
			return cd.Respond(Response{Key: "reminder/set.past"})
		}
		payload, err := json.Marshal(database.ScheduledTaskReminderPayload{
			ChannelID: cd.ChannelID,
			Reason:    message,
		})
		if err != nil {
			return err
		}
		task := database.ScheduledTask{
			GuildID:      cd.GuildID,
			TaskType:     database.TaskTypeReminder,
			UserID:       cd.Member.User.ID,
			ProcessAfter: parsedTime.Time,
			Payload:      payload,
		}
		if res := database.Database.Create(&task); res.Error != nil {
			return res.Error
		}
		cd.Log.Info().Uint("id", task.ID).Msg("Created reminder")
		return cd.Respond(Response{Key: "reminder/set.success", Vars: &i18n.Vars{
			"time": fmt.Sprintf("<t:%d:f>", parsedTime.Time.Unix()),
			"id":   task.ID,
		}})
	},
}

var reminderCancel = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "cancel",
		Options: []*dg.ApplicationCommandOption{
			{Name: "id", Type: dg.ApplicationCommandOptionInteger, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		id := cd.Option("id").UintValue()
		task := database.ScheduledTask{ID: uint(id), GuildID: cd.GuildID, TaskType: database.TaskTypeReminder}
		if res := database.Database.Delete(&task); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return cd.Respond(Response{Key: "reminder/cancel.missing"})
			}
			return res.Error
		}
		return cd.Respond(Response{Key: "reminder/clear.success"})
	},
}

var reminderList = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "list"},
	CommandHandler: func(cd *CommandData) error {
		var tasks []*database.ScheduledTask
		if res := database.Database.Where(
			&database.ScheduledTask{GuildID: cd.GuildID, TaskType: database.TaskTypeReminder},
			datatypes.JSONQuery("payload").Equals(cd.ChannelID, "ChannelID"),
		).Order("process_after").Limit(11).Find(&tasks); res.Error != nil {
			return res.Error
		} else if res.RowsAffected == 0 {
			return cd.Respond(Response{Key: "reminder/list.empty"})
		}
		embed := &dg.MessageEmbed{
			Fields: lo.Map(lo.Slice(tasks, 0, 10), func(t *database.ScheduledTask, _ int) *dg.MessageEmbedField {
				var payload database.ScheduledTaskReminderPayload
				var userName string
				if err := json.Unmarshal(t.Payload, &payload); err != nil {
					cd.Log.Error().Any("payload", t.Payload).Msg("Failed to unmarshal JSON for scheduled task when loading reminder. Skipping.")
				}
				if user, err := cd.GuildMember(cd.GuildID, t.UserID); err == nil {
					userName = user.DisplayName()
				} else {
					cd.Log.Warn().Str("guild", cd.GuildID).Str("user", t.UserID).Err(err).Msg("Failed to get user for reminder")
					userName = "Unknown User"
				}
				return &dg.MessageEmbedField{
					Name:  fmt.Sprintf("[#%d] %s", t.ID, userName),
					Value: payload.Reason,
				}
			}),
		}
		if len(tasks) > 10 {
			embed.Footer = &dg.MessageEmbedFooter{Text: i18n.Get(*cd.GuildLocale, "reminders/list.hasMore")}
		}
		return cd.InteractionRespond(cd.Interaction, &dg.InteractionResponse{
			Type: dg.InteractionResponseChannelMessageWithSource,
			Data: &dg.InteractionResponseData{
				Embeds: []*dg.MessageEmbed{embed},
				Flags:  dg.MessageFlagsEphemeral,
			},
		})
	},
}
