package events

import (
	"slices"
	"snoozybot/internal/commands"
	"snoozybot/internal/config"
	"snoozybot/internal/database"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func roleMessageMetricsHandler(d EventData[dg.MessageCreate]) error {
	// Make sure regulars role is turned on for the server first
	roleId, err := config.RolesRegularsRoleID.Get(d.Event.GuildID).Value()
	if err != nil || roleId == "" {
		return nil
	}
	roleIdStr := string(roleId)

	// Ignore bots
	if d.Event.Author == nil || d.Event.Author.Bot {
		return nil
	}

	// Only if user doesnt already have the role
	if slices.Contains(d.Event.Member.Roles, roleIdStr) {
		return nil
	}

	// Insert or update metric
	result := database.Database.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "guild_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"message_count":              gorm.Expr("message_metrics.message_count + 1"),
			"distinct_days":              gorm.Expr("case when message_metrics.last_distinct_day_boundary < current_timestamp - interval '1 day' then message_metrics.distinct_days + 1 else message_metrics.distinct_days end"),
			"last_distinct_day_boundary": gorm.Expr("case when message_metrics.last_distinct_day_boundary < current_timestamp - interval '1 day' then current_timestamp else message_metrics.last_distinct_day_boundary end"),
		}),
	}).Create(&database.MessageMetric{
		UserID:                  d.Event.Author.ID,
		GuildID:                 d.Event.GuildID,
		MessageCount:            1,
		DistinctDays:            1,
		LastDistinctDayBoundary: time.Now(),
	})

	if result.Error != nil {
		d.Logger.Error().Err(result.Error).Msg("Failed to update message metrics")
	}

	// If the role is automated, check if the user should be given the role
	isAuto, err := config.RolesRegularsAutoAssign.Get(d.Event.GuildID).Value()
	if err != nil || !isAuto {
		return err
	}
	d.Event.Member.User = d.Event.Author // the lib doesnt fill this
	commands.TryAssignRegularsRole(d.Event.GuildID, d.Event.Member, d.Logger, d.Session)
	return nil
}
