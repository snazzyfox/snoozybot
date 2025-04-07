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
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("request returned redirect")
	},
}

func twitchStreamGuildAvailable(s *dg.Session, m *dg.GuildCreate) {
	// Sync all users presence with the role on bot start

	streamingRoleID, _ := config.TwitchLiveRoleID.Get(m.Guild.ID).Value()
	if streamingRoleID == "" {
		return
	}
	log.Info().Str("guild", m.Guild.ID).Msg("Syncing twitch stream presence for guild on startup.")

	var live []string
	log.Debug().Str("guild", m.Guild.ID).Any("presences", m.Guild.Presences).Msg("Presences") // TODO REMOVE
userPresence:
	for _, presence := range m.Guild.Presences {
		// missing is unknown; don't touch it
		if presence.Activities != nil {
			// todo check if user is allowed to notify
			for _, activity := range presence.Activities {
				if activity.Type == dg.ActivityTypeStreaming && strings.HasPrefix(activity.URL, "https://www.twitch.tv/") {
					// is streaming
					live = append(live, activity.URL[22:])
					s.GuildMemberRoleAdd(m.Guild.ID, presence.User.ID, string(streamingRoleID))
					log.Info().Str("user", presence.User.Username).Str("channel", activity.URL[22:]).Msg("User is streaming, adding role")
					break userPresence
				}
			}
			// got to the end, not streaming
			log.Debug().Str("user", presence.User.Username).Msg("User is not streaming, removing role")
			s.GuildMemberRoleRemove(m.Guild.ID, presence.User.ID, string(streamingRoleID))
		}
	}
	log.Info().Strs("live", live).Str("guild", m.Guild.ID).Msg("Finished processing initial presence data.")
	twitch.GetStreams(live) // result ignored; just to update cache
}

func twitchStreamPresenceUpdate(s *dg.Session, m *dg.PresenceUpdate) {
	log.Debug().Any("presence", m).Msg("Received presence update.")

	streamingRoleID, _ := config.TwitchLiveRoleID.Get(m.GuildID).Value()
	channelID, _ := config.TwitchLiveChannelID.Get(m.GuildID).Value()
	templateText, _ := config.TwitchLiveTemplate.Get(m.GuildID).Value()
	if streamingRoleID == "" && channelID == "" {
		return
	}

	// check if user is allowed to notify. If the role list is empty anyone can notify
	if member, err := s.GuildMember(m.GuildID, m.User.ID); err != nil {
		return
	} else {
		eligibleRoleIDs, _ := config.TwitchLiveEligibleRoleIDs.Get(m.GuildID).Value()
		if len(eligibleRoleIDs) > 0 && lo.None(lo.Map(eligibleRoleIDs, func(id json.Number, _ int) string { return string(id) }), member.Roles) {
			return
		}
	}

	if m.Activities != nil {
		for _, activity := range m.Activities {
			if activity.Type == dg.ActivityTypeStreaming {
				if streamingRoleID != "" {
					log.Info().Str("user", m.User.ID).Str("guild", m.GuildID).Msg("User is streaming, adding role")
					s.GuildMemberRoleAdd(m.GuildID, m.User.ID, string(streamingRoleID))
				}
				if channelID != "" {
					twitchChannel := activity.URL[22:]
					go func(twitchChannel string, channelID string, userID string) {
						log.Debug().Str("channel", twitchChannel).Msg("Getting stream info")
						if stream, isNew, err := twitch.AttemptGetStream(twitchChannel); err != nil {
							log.Error().Str("channel", twitchChannel).Err(err).Msg("Failed to get stream info")
							return
						} else if isNew {
							content := i18n.TemplateString(lo.Must(template.New("twitch_live").Parse(templateText)), &i18n.Vars{"user": m.User.Mention()})
							embed := generateStreamNotificationEmbed(&stream)
							log.Info().Str("user", userID).Str("twitch", twitchChannel).Str("discordChanne", channelID).Msg("Sending stream notification")
							s.ChannelMessageSendComplex(channelID, &dg.MessageSend{Content: content, Embed: embed})
						} else {
							log.Debug().Str("channel", twitchChannel).Msg("Stream is not new, skipping notification")
						}
					}(twitchChannel, string(channelID), m.User.ID)
				}
				return
			}
		}
		// none of the activities are streaming
		if streamingRoleID != "" {
			log.Info().Str("user", m.User.ID).Msg("User is not streaming, removing role")
			s.GuildMemberRoleRemove(m.GuildID, m.User.ID, string(streamingRoleID))
		}
	}
}

func generateStreamNotificationEmbed(stream *helix.Stream) *dg.MessageEmbed {
	// Wait for the stream thumbnail to be available
	thumbnailURL := strings.Replace(stream.ThumbnailURL, "{width}x{height}", "1024x576", 1)
	lo.AttemptWithDelay(20, 30*time.Second, func(index int, duration time.Duration) error {
		// errors if a redirect is encountered. Twitch returns a 302 to the generic image if one isnt available.
		res, err := noRedirectClient.Head(thumbnailURL)
		log.Debug().Err(err).Str("url", thumbnailURL).Int("status", res.StatusCode).Msg("Attempted to get thumbnail.")
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
