package events

import (
	"snoozybot/internal/database"

	dg "github.com/bwmarrin/discordgo"
)

func memberLeaveCleanup(d EventData[dg.GuildMemberRemove]) error {
	userId := d.Event.User.ID
	d.Logger.Info().Str("guild", d.Event.GuildID).Str("user", userId).Msg("Cleaning up data because user left server.")

	if res := database.Database.Where(&database.Quote{GuildID: d.Event.GuildID, UserID: userId}).Delete(&database.Quote{}); res.Error != nil {
		d.Logger.Warn().Err(res.Error).Str("guild", d.Event.GuildID).Str("user", userId).Msg("Failed to delete quotes.")
	}
	if res := database.Database.Where(&database.ScheduledTask{GuildID: d.Event.GuildID, UserID: userId}).Delete(&database.ScheduledTask{}); res.Error != nil {
		d.Logger.Warn().Err(res.Error).Str("guild", d.Event.GuildID).Str("user", userId).Msg("Failed to delete scheduled tasks.")
	}
	if res := database.Database.Where(&database.MessageMetric{GuildID: d.Event.GuildID, UserID: userId}).Delete(&database.MessageMetric{}); res.Error != nil {
		d.Logger.Warn().Err(res.Error).Str("guild", d.Event.GuildID).Str("user", userId).Msg("Failed to delete message metrics.")
	}
	return nil
}
