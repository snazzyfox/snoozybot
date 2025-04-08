package events

import (
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

const cooldownDuration = time.Duration(30 * time.Minute)
const sleepTime = time.Duration(6 * time.Hour)

func bedtimeHandler(d EventData[dg.MessageCreate]) error {
	if d.Event.Author.Bot {
		return nil
	}
	user, err := database.GetUser(d.Event.Author.ID)
	if err != nil {
		return err
	}
	if user.Timezone == nil || user.Bedtime == nil {
		d.Logger.Debug().Any("user", user).Msg("User does not have timezone or bedtime set, ignoring.")
		return nil
	}
	now := time.Now().In(time.UTC)

	// Skip if just recetly notified
	if user.LastBedtimeNotified != nil && now.Sub(*user.LastBedtimeNotified) < cooldownDuration {
		d.Logger.Debug().Any("user", user).Msg("User within bedtime notification cooldown.")
		return nil
	}

	// Get the current time in the user's timezone.
	tz, err := time.LoadLocation(*user.Timezone)
	if err != nil {
		d.Logger.Warn().Err(err).Msg("Failed to parse timezone for user. Ignoring operation.")
		return nil
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
		return nil
	}
	d.Logger.Info().Any("user", user).Msg("Sending bedtime notification")
	var key string
	if timeSinceBed < (sleepTime / 2) { // First half of sleep period
		key = "my/bedtime/notifs.late"
	} else { // Second half of sleep period
		key = "my/bedtime/notifs.early"
	}
	locale := dg.Locale(lo.Must(d.Session.Guild(d.Event.GuildID)).PreferredLocale)
	_, err = d.Session.ChannelMessageSend(d.Event.ChannelID, i18n.Get(locale, key, &i18n.Vars{"user": d.Event.Author.Mention()}))
	if err != nil {
		return err
	}

	// Remember we've sent the message so we dont spam them
	user.LastBedtimeNotified = &now
	result := database.Database.Save(user)
	if result.Error != nil {
		return result.Error
	}
	database.UserCache.Add(d.Event.Author.ID, *user)
	return nil
}
