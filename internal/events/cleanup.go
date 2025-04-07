package events

import (
	"snoozybot/internal/database"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

func memberLeaveCleanup(s *dg.Session, m *dg.GuildMemberRemove) {
	userId := m.User.ID
	log.Info().Str("guild", m.GuildID).Str("user", userId).Msg("Cleaning up data because user left server.")

	if res := database.Database.Where(&database.Quote{GuildID: m.GuildID, UserID: userId}).Delete(&database.Quote{}); res.Error != nil {
		log.Warn().Err(res.Error).Str("guild", m.GuildID).Str("user", userId).Msg("Failed to delete quotes.")
	}
	if res := database.Database.Where(&database.ScheduledTask{GuildID: m.GuildID, UserID: userId}).Delete(&database.ScheduledTask{}); res.Error != nil {
		log.Warn().Err(res.Error).Str("guild", m.GuildID).Str("user", userId).Msg("Failed to delete scheduled tasks.")
	}
	if res := database.Database.Where(&database.MessageMetric{GuildID: m.GuildID, UserID: userId}).Delete(&database.MessageMetric{}); res.Error != nil {
		log.Warn().Err(res.Error).Str("guild", m.GuildID).Str("user", userId).Msg("Failed to delete message metrics.")
	}
}
