package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"snoozybot/internal/config"
	"snoozybot/internal/i18n"
	"snoozybot/internal/twitch"
	"strings"
	"text/template"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/nicklaw5/helix/v2"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("request returned redirect")
	},
}

func twitchStreamGuildAvailable(d EventData[dg.GuildCreate]) error {
	// Sync all users presence with the role on bot start

	streamingRoleID, _ := config.TwitchLiveRoleID.Get(d.Event.Guild.ID).Value()
	if streamingRoleID == "" {
		return nil
	}
	d.Logger.Info().Str("guild", d.Event.Guild.ID).Msg("Syncing twitch stream presence for guild on startup.")

	var live []string
	d.Logger.Debug().Str("guild", d.Event.Guild.ID).Any("presences", d.Event.Guild.Presences).Msg("Presences") // TODO REMOVE
userPresence:
	for _, presence := range d.Event.Guild.Presences {
		// missing is unknown; don't touch it
		if presence.Activities != nil {
			// todo check if user is allowed to notify
			for _, activity := range presence.Activities {
				if activity.Type == dg.ActivityTypeStreaming && strings.HasPrefix(activity.URL, "https://www.twitch.tv/") {
					// is streaming
					live = append(live, activity.URL[22:])
					d.Session.GuildMemberRoleAdd(d.Event.Guild.ID, presence.User.ID, string(streamingRoleID))
					d.Logger.Info().Str("user", presence.User.ID).Str("channel", activity.URL[22:]).Msg("User is streaming, adding role")
					break userPresence
				}
			}
			// got to the end, not streaming
			d.Logger.Debug().Str("user", presence.User.Username).Msg("User is not streaming, removing role")
			d.Session.GuildMemberRoleRemove(d.Event.Guild.ID, presence.User.ID, string(streamingRoleID))
		}
	}
	d.Logger.Info().Strs("live", live).Str("guild", d.Event.Guild.ID).Msg("Finished processing initial presence data.")
	twitch.GetStreams(live) // result ignored; just to update cache
	return nil
}

func twitchStreamPresenceUpdate(d EventData[dg.PresenceUpdate]) error {
	d.Logger.Debug().Any("presence", d.Event).Msg("Received presence update.")

	streamingRoleID, _ := config.TwitchLiveRoleID.Get(d.Event.GuildID).Value()
	channelID, _ := config.TwitchLiveChannelID.Get(d.Event.GuildID).Value()
	templateText, _ := config.TwitchLiveTemplate.Get(d.Event.GuildID).Value()
	if streamingRoleID == "" && channelID == "" {
		return nil
	}

	// check if user is allowed to notify. If the role list is empty anyone can notify
	if member, err := d.Session.GuildMember(d.Event.GuildID, d.Event.User.ID); err != nil {
		return err
	} else {
		eligibleRoleIDs, _ := config.TwitchLiveEligibleRoleIDs.Get(d.Event.GuildID).Value()
		if len(eligibleRoleIDs) > 0 && lo.None(lo.Map(eligibleRoleIDs, func(id json.Number, _ int) string { return string(id) }), member.Roles) {
			return nil
		}
	}

	if d.Event.Activities != nil {
		for _, activity := range d.Event.Activities {
			if activity.Type == dg.ActivityTypeStreaming && strings.HasPrefix(activity.URL, "https://www.twitch.tv/") {
				if streamingRoleID != "" {
					d.Logger.Debug().Str("user", d.Event.User.ID).Str("guild", d.Event.GuildID).Msg("User is streaming, adding role")
					d.Session.GuildMemberRoleAdd(d.Event.GuildID, d.Event.User.ID, string(streamingRoleID))
				}
				if channelID != "" {
					twitchChannel := activity.URL[22:]
					go func(twitchChannel string, channelID string, userID string) {
						d.Logger.Debug().Str("channel", twitchChannel).Msg("Getting stream info")
						if stream, isNew, err := twitch.AttemptGetStream(twitchChannel); err != nil {
							d.Logger.Error().Str("channel", twitchChannel).Err(err).Msg("Failed to get stream info")
							return
						} else if isNew {
							content := i18n.TemplateString(lo.Must(template.New("twitch_live").Parse(templateText)), &i18n.Vars{"user": d.Event.User.Mention()})
							embed := generateStreamNotificationEmbed(&stream, d.Logger)
							d.Logger.Info().Str("user", d.Event.User.ID).Str("twitch", twitchChannel).Str("channel", channelID).Msg("Sending stream notification")
							d.Session.ChannelMessageSendComplex(channelID, &dg.MessageSend{Content: content, Embed: embed})
						} else {
							d.Logger.Debug().Str("channel", twitchChannel).Msg("Stream is not new, skipping notification")
						}
					}(twitchChannel, string(channelID), d.Event.User.ID)
				}
				return nil
			}
		}
		// none of the activities are streaming
		if streamingRoleID != "" {
			d.Logger.Debug().Str("user", d.Event.User.ID).Msg("User is not streaming, removing role")
			d.Session.GuildMemberRoleRemove(d.Event.GuildID, d.Event.User.ID, string(streamingRoleID))
		}
	}
	return nil
}

func generateStreamNotificationEmbed(stream *helix.Stream, logger *zerolog.Logger) *dg.MessageEmbed {
	// Wait for the stream thumbnail to be available
	thumbnailURL := strings.Replace(stream.ThumbnailURL, "{width}x{height}", "1024x576", 1)
	lo.AttemptWithDelay(20, 30*time.Second, func(index int, duration time.Duration) error {
		// errors if a redirect is encountered. Twitch returns a 302 to the generic image if one isnt available.
		res, err := noRedirectClient.Head(thumbnailURL)
		logger.Debug().Err(err).Str("url", thumbnailURL).Int("status", res.StatusCode).Msg("Attempted to get thumbnail.")
		return err
	})
	userProfileImage, _ := twitch.GetProfileImageURL(stream.UserLogin)
	return &dg.MessageEmbed{
		Title:       stream.Title,
		Description: stream.GameName,
		URL:         "https://www.twitch.tv/" + stream.UserLogin,
		Author:      &dg.MessageEmbedAuthor{Name: stream.UserLogin},
		Thumbnail:   &dg.MessageEmbedThumbnail{URL: userProfileImage},
		Timestamp:   stream.StartedAt.Format(time.RFC3339),
		Footer:      &dg.MessageEmbedFooter{Text: strings.Join(stream.Tags, ", ")},
		Image:       &dg.MessageEmbedImage{URL: thumbnailURL},
	}
}
