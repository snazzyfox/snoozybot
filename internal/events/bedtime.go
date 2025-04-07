package events

import (
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

const cooldown = time.Duration(30 * time.Minute)
const sleepTime = time.Duration(6 * time.Hour)

func bedtimeHandler(s *dg.Session, m *dg.MessageCreate) {
	if m.Author.Bot {
		return
	}
	user, err := database.GetUser(m.Author.ID)
	if err != nil {
		log.Error().Err(err).Any("user", m.Author).Msg("Failed to get user")
		return
	}
	if user.Timezone == nil || user.Bedtime == nil {
		log.Debug().Any("user", user).Msg("User does not have timezone or bedtime set, ignoring.")
		return
	}
	now := time.Now().In(time.UTC)

	// Skip if just recetly notified
	if user.LastBedtimeNotified != nil && now.Sub(*user.LastBedtimeNotified) < cooldown {
		log.Debug().Any("user", user).Msg("User within bedtime notification cooldown.")
		return
	}

	// Get the current time in the user's timezone.
	tz, err := time.LoadLocation(*user.Timezone)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse timezone for user. Ignoring operation.")
		return
	}

	// Get the current date's bedtime for the user.
	userNow := now.In(tz)
	userBedtime := time.Date(userNow.Year(), userNow.Month(), userNow.Day(), 0, 0, 0, 0, tz).Add(time.Duration(*user.Bedtime))
	if userBedtime.After(userNow) {
		// That bedtime is in the future. Could be 1AM when the bedtime is 11PM, when we're technically within range. Go back a day
		userBedtime = userBedtime.Add(-24 * time.Hour)
	}

	timeSinceBed := userNow.Sub(userBedtime)
	if timeSinceBed > sleepTime {
		return
	}
	log.Info().Any("user", user).Msg("Sending bedtime notification")
	var key string
	if timeSinceBed < (sleepTime / 2) { // First half of sleep period
		key = "my/bedtime/notifs.late"
	} else { // Second half of sleep period
		key = "my/bedtime/notifs.early"
	}
	locale := dg.Locale(lo.Must(s.Guild(m.GuildID)).PreferredLocale)
	_, err = s.ChannelMessageSend(m.ChannelID, i18n.Get(locale, key, &i18n.Vars{"user": m.Author.Mention()}))
	if err != nil {
		log.Error().Err(err).Any("user", user).Msg("Failed to send bedtime message")
	}

	// Remember we've sent the message so we dont spam them
	user.LastBedtimeNotified = &now
	result := database.Database.Save(user)
	if result.Error != nil {
		log.Error().Err(result.Error).Any("user", user).Msg("Failed to update bedtime cooldown time for user")
	}
	database.UserCache.Add(m.Author.ID, *user)
}
