package tasks

import (
	"encoding/json"
	"snoozybot/internal/config"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"

	"gorm.io/gorm/clause"
)

var storedScheduledTask = PeriodicTask{
	Name:     "storedScheduledTask",
	Interval: 1 * time.Minute,
	TaskHandler: func(ctx *TaskData) error {
		var dueTasks []database.ScheduledTask
		// Get all tasks that are due to be processed and delete at the same time to prevent duplicate processing
		database.Database.Clauses(clause.Returning{}).Where("process_after < ?", time.Now()).Delete(&dueTasks)

		// Start gorountines for each task
		for _, task := range dueTasks {
			ctx.Logger = ctx.Logger.With().Uint("task", task.ID).Uint("type", uint(task.TaskType)).Logger()
			switch task.TaskType {
			case database.TaskTypeReminder:
				go processReminder(&task, ctx)
			case database.TaskTypeBirthday:
				go processBirthday(&task, ctx)
			case database.TaskTypeRemoveRole:
				go processRemoveRole(&task, ctx)
			default:
				ctx.Logger.Warn().Uint("type", uint(task.TaskType)).Msg("Unknown task type")
			}
		}
		return nil
	},
}

func _getTaskInfo(task *database.ScheduledTask, ctx *TaskData) (*dg.Session, *dg.Guild, *dg.Member) {
	bot, ok := ctx.BotManager.GuildBots[task.GuildID]
	if !ok {
		ctx.Logger.Warn().Str("guild", task.GuildID).Msg("Bot not found for guild")
		return nil, nil, nil
	}
	guild, err := bot.Guild(task.GuildID)
	if err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to get guild")
		return nil, nil, nil
	}
	member, err := bot.GuildMember(task.GuildID, task.UserID)
	if err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to get guild member")
		return nil, nil, nil
	}
	return bot, guild, member
}

func processReminder(task *database.ScheduledTask, ctx *TaskData) error {
	bot, guild, member := _getTaskInfo(task, ctx)
	if bot == nil || guild == nil || member == nil {
		return nil
	}
	var payload database.ScheduledTaskReminderPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to parse scheduled task payload")
		return nil
	}
	text := i18n.Get(dg.Locale(guild.PreferredLocale), "reminder/notif", &i18n.Vars{"name": member.Mention(), "content": payload.Reason})
	if _, err := bot.ChannelMessageSendComplex(payload.ChannelID, &dg.MessageSend{
		Content:         text,
		AllowedMentions: &dg.MessageAllowedMentions{Parse: []dg.AllowedMentionType{dg.AllowedMentionTypeUsers}},
	}); err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to send reminder message")
		return err
	}
	return nil
}

func processBirthday(task *database.ScheduledTask, ctx *TaskData) error {
	bot, guild, member := _getTaskInfo(task, ctx)
	if bot == nil || guild == nil || member == nil {
		return nil
	}
	var payload database.ScheduledTaskBirthdayPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to parse scheduled task payload")
		return nil
	}
	channelId, err := config.ProfileBirthdayChannel.Get(task.GuildID).Value()
	if err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to get birthday channel")
		return nil
	}
	text := i18n.Get(dg.Locale(guild.PreferredLocale), "my/birthday/notif", &i18n.Vars{"name": member.Mention()})
	_, err = bot.ChannelMessageSendComplex(string(channelId), &dg.MessageSend{
		Content:         text,
		AllowedMentions: &dg.MessageAllowedMentions{Parse: []dg.AllowedMentionType{dg.AllowedMentionTypeUsers}},
	})
	if err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to send birthday message")
		return nil
	}

	// schedule the next birthday
	nextBirthday := task.ProcessAfter.AddDate(1, 0, 0)
	database.Database.Create(&database.ScheduledTask{
		GuildID:      task.GuildID,
		TaskType:     database.TaskTypeBirthday,
		ProcessAfter: nextBirthday,
		UserID:       task.UserID,
	})
	return nil
}

func processRemoveRole(task *database.ScheduledTask, ctx *TaskData) error {
	bot, guild, member := _getTaskInfo(task, ctx)
	if bot == nil || guild == nil || member == nil {
		return nil
	}
	var payload database.ScheduledTaskRemoveRolePayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to parse scheduled task payload")
		return nil
	}
	if err := bot.GuildMemberRoleRemove(guild.ID, member.User.ID, payload.RoleID); err != nil {
		ctx.Logger.Error().Err(err).Msg("Failed to remove role")
		return err
	}
	ctx.Logger.Info().Str("guild", guild.ID).Str("user", member.User.ID).Str("role", payload.RoleID).Msg("Removed temporary role")
	return nil
}
