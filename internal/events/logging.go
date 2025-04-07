package events

import (
	"errors"
	"fmt"
	"snoozybot/internal/config"
	"strings"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

var cache = expirable.NewLRU[string, *dg.Message](100_000, nil, 7*24*time.Hour)

type auditKey struct {
	action dg.AuditLogAction
	key    string
}

var audit = expirable.NewLRU[auditKey, *dg.AuditLogEntry](1_000, nil, 5*time.Second)

func logMessageCreate(s *dg.Session, ev *dg.MessageCreate) {
	if config.LogsChannelID.Get(ev.GuildID).Exists() && ev.Author != nil && !ev.Author.Bot {
		log.Trace().Any("event", ev.Message).Msg("Message Created")
		cache.Add(ev.ID, ev.Message)
	}
}

func logMessageUpdate(s *dg.Session, ev *dg.MessageUpdate) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		if cached, ok := cache.Get(ev.ID); ok {
			log.Debug().Any("event", ev).Any("cached", cached).Msg("Message updated, found in cache.")
			if ev.Author != nil && !ev.Author.Bot && ev.EditedTimestamp != nil {
				s.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
					Title: "Message Updated",
					Fields: []*dg.MessageEmbedField{
						{Name: "Channel", Value: fmt.Sprintf("<#%s>", ev.ChannelID), Inline: true},
						{Name: "Author", Value: fmt.Sprintf("<@%s>", cached.Author.ID), Inline: true},
						{Name: "Originally Sent", Value: fmt.Sprintf("<t:%d:f>", cached.Timestamp.Unix()), Inline: true},
						{Name: "Message URL", Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ev.GuildID, ev.ChannelID, ev.ID)},
						{Name: "Previous Content", Value: cached.Content},
						{Name: "New Content", Value: ev.Content},
					},
				})
			}
		} else {
			log.Debug().Any("event", ev).Msg("Message updated, but not found in cache.")
		}
		cache.Add(ev.ID, ev.Message)
	}
}

func logMessageDelete(s *dg.Session, ev *dg.MessageDelete) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		if cached, ok := cache.Get(ev.ID); ok {
			log.Debug().Any("cached", cached).Msg("Message deleted, found in cache.")
			auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMessageDelete, key: ev.ChannelID + cached.Author.ID})
			s.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
				Title: "Message Deleted",
				Fields: []*dg.MessageEmbedField{
					{Name: "Channel", Value: fmt.Sprintf("<#%s>", ev.ChannelID), Inline: true},
					{Name: "Author", Value: fmt.Sprintf("<@%s>", cached.Author.ID), Inline: true},
					{Name: "Sent At", Value: fmt.Sprintf("<t:%d:f>", cached.Timestamp.Unix()), Inline: true},
					{Name: "Message URL", Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ev.GuildID, ev.ChannelID, ev.ID)},
					{Name: "Content", Value: cached.Content},
					{Name: "Attachments", Value: lo.CoalesceOrEmpty(strings.Join(lo.Map(cached.Attachments, func(attachment *dg.MessageAttachment, _ int) string {
						return attachment.URL
					}), ", "), "None")},
				},
			}, auditEntry))
			cache.Remove(ev.ID)
		} else {
			log.Debug().Any("event", ev).Msg("Message deleted, but not found in cache.")
		}
	}
}

func logBan(s *dg.Session, ev *dg.GuildBanAdd) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		log.Debug().Any("event", ev).Msg("Guild Ban Added")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberBanAdd, key: ev.User.ID})
		s.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
			Title: "User Banned",
			Color: 0x880000,
			Fields: []*dg.MessageEmbedField{
				{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
			},
		}, auditEntry))
	}
}

func logUnban(s *dg.Session, ev *dg.GuildBanRemove) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		log.Debug().Any("event", ev).Msg("Guild Ban Removed")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberBanRemove, key: ev.User.ID})
		s.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
			Title: "User Unbanned",
			Color: 0x008800,
			Fields: []*dg.MessageEmbedField{
				{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
			},
			Thumbnail: &dg.MessageEmbedThumbnail{URL: ev.User.AvatarURL("128")},
		}, auditEntry))
	}
}

func logLeave(s *dg.Session, ev *dg.GuildMemberRemove) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		log.Debug().Any("event", ev).Msg("User Left Guild")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberKick, key: ev.User.ID})
		if auditEntry != nil {
			s.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
				Title: "User Kicked",
				Color: 0x000088,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
				},
				Thumbnail: &dg.MessageEmbedThumbnail{URL: ev.User.AvatarURL("128")},
			}, auditEntry))
		} else {
			s.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Left",
				Color: 0x000088,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
				},
			})
		}

	}
}

func logTimeout(s *dg.Session, ev *dg.GuildMemberUpdate) {
	if logChannel, err := config.LogsChannelID.Get(ev.GuildID).Value(); err == nil {
		if ev.CommunicationDisabledUntil != nil && ev.BeforeUpdate != nil && ev.CommunicationDisabledUntil != ev.BeforeUpdate.CommunicationDisabledUntil {
			// Timeout set
			s.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Timed Out",
				Color: 0x888800,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
					{Name: "Until", Value: fmt.Sprintf("<t:%d:f>", ev.CommunicationDisabledUntil.Unix()), Inline: true},
				},
			})

		} else if ev.CommunicationDisabledUntil == nil && ev.BeforeUpdate != nil && ev.BeforeUpdate.CommunicationDisabledUntil != nil {
			// Timeout removed
			s.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Timeout Removed",
				Color: 0x008888,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", ev.User.ID), Inline: true},
				},
			})
		}
	}
}

func logAuditLog(s *dg.Session, ev *dg.GuildAuditLogEntryCreate) {
	switch *ev.ActionType {
	case dg.AuditLogActionMessageDelete, dg.AuditLogActionMessageBulkDelete:
		audit.Add(auditKey{action: *ev.ActionType, key: ev.Options.ChannelID + ev.TargetID}, ev.AuditLogEntry)
	case dg.AuditLogActionMemberBanAdd, dg.AuditLogActionMemberBanRemove, dg.AuditLogActionMemberKick:
		audit.Add(auditKey{action: *ev.ActionType, key: ev.TargetID}, ev.AuditLogEntry)
	case dg.AuditLogActionMemberUpdate:
		if len(ev.Changes) > 0 && *ev.Changes[0].Key == dg.AuditLogChangeKeyCommunicationDisabledUntil {
			audit.Add(auditKey{action: *ev.ActionType, key: ev.TargetID}, ev.AuditLogEntry)
		}
	}
}

func waitForAuditLog(key auditKey) *dg.AuditLogEntry {
	var auditEntry *dg.AuditLogEntry
	lo.AttemptWithDelay(10, 500*time.Millisecond, func(index int, duration time.Duration) error {
		if res, ok := audit.Get(key); ok {
			auditEntry = res
			return nil
		} else {
			return errors.New("audit log not found")
		}
	})
	return auditEntry
}

func addAuditLogFields(embed *dg.MessageEmbed, auditEntry *dg.AuditLogEntry) *dg.MessageEmbed {
	if auditEntry == nil {
		return embed
	}

	embed.Fields = append(embed.Fields, &dg.MessageEmbedField{
		Name:   "Performed By",
		Value:  fmt.Sprintf("<@%s>", auditEntry.UserID),
		Inline: true,
	}, &dg.MessageEmbedField{
		Name:  "Reason",
		Value: lo.CoalesceOrEmpty(auditEntry.Reason, "Not provided"),
	})
	return embed
}
