package events

import (
	"errors"
	"fmt"
	"snoozybot/internal/config"
	"strings"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/samber/lo"
)

var cache = expirable.NewLRU[string, *dg.Message](100_000, nil, 7*24*time.Hour)

type auditKey struct {
	action dg.AuditLogAction
	key    string
}

var audit = expirable.NewLRU[auditKey, *dg.AuditLogEntry](1_000, nil, 5*time.Second)

func logMessageCreate(d EventData[dg.MessageCreate]) error {
	if config.LogsChannelID.Get(d.Event.GuildID).Exists() && d.Event.Author != nil && !d.Event.Author.Bot {
		d.Logger.Trace().Any("event", d.Event.Message).Msg("Message Created")
		cache.Add(d.Event.Message.ID, d.Event.Message)
	}
	return nil
}

func logMessageUpdate(d EventData[dg.MessageUpdate]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		if cached, ok := cache.Get(d.Event.Message.ID); ok {
			d.Logger.Debug().Any("event", d.Event).Any("cached", cached).Msg("Message updated, found in cache.")
			if d.Event.Author != nil && !d.Event.Author.Bot && d.Event.EditedTimestamp != nil {
				_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
					Title: "Message Updated",
					Fields: []*dg.MessageEmbedField{
						{Name: "Channel", Value: fmt.Sprintf("<#%s>", d.Event.ChannelID), Inline: true},
						{Name: "Author", Value: fmt.Sprintf("<@%s>", cached.Author.ID), Inline: true},
						{Name: "Originally Sent", Value: fmt.Sprintf("<t:%d:f>", cached.Timestamp.Unix()), Inline: true},
						{Name: "Message URL", Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", d.Event.GuildID, d.Event.ChannelID, d.Event.Message.ID)},
						{Name: "Previous Content", Value: cached.Content},
						{Name: "New Content", Value: d.Event.Content},
					},
				})
				return err
			}
		} else {
			d.Logger.Debug().Any("event", d.Event).Msg("Message updated, but not found in cache.")
		}
		cache.Add(d.Event.Message.ID, d.Event.Message)
	}
	return nil
}

func logMessageDelete(d EventData[dg.MessageDelete]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		if cached, ok := cache.Get(d.Event.Message.ID); ok {
			d.Logger.Debug().Any("cached", cached).Msg("Message deleted, found in cache.")
			auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMessageDelete, key: d.Event.ChannelID + cached.Author.ID})
			_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
				Title: "Message Deleted",
				Fields: []*dg.MessageEmbedField{
					{Name: "Channel", Value: fmt.Sprintf("<#%s>", d.Event.ChannelID), Inline: true},
					{Name: "Author", Value: fmt.Sprintf("<@%s>", cached.Author.ID), Inline: true},
					{Name: "Sent At", Value: fmt.Sprintf("<t:%d:f>", cached.Timestamp.Unix()), Inline: true},
					{Name: "Message URL", Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", d.Event.GuildID, d.Event.ChannelID, d.Event.Message.ID)},
					{Name: "Content", Value: cached.Content},
					{Name: "Attachments", Value: lo.CoalesceOrEmpty(strings.Join(lo.Map(cached.Attachments, func(attachment *dg.MessageAttachment, _ int) string {
						return attachment.URL
					}), ", "), "None")},
				},
			}, auditEntry))
			cache.Remove(d.Event.Message.ID)
			return err
		} else {
			d.Logger.Debug().Any("event", d.Event).Msg("Message deleted, but not found in cache.")
		}
	}
	return nil
}

func logBan(d EventData[dg.GuildBanAdd]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		d.Logger.Debug().Any("event", d.Event).Msg("Guild Ban Added")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberBanAdd, key: d.Event.User.ID})
		_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
			Title: "User Banned",
			Color: 0x880000,
			Fields: []*dg.MessageEmbedField{
				{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
			},
		}, auditEntry))
		return err
	}
	return nil
}

func logUnban(d EventData[dg.GuildBanRemove]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		d.Logger.Debug().Any("event", d.Event).Msg("Guild Ban Removed")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberBanRemove, key: d.Event.User.ID})
		_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
			Title: "User Unbanned",
			Color: 0x008800,
			Fields: []*dg.MessageEmbedField{
				{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
			},
			Thumbnail: &dg.MessageEmbedThumbnail{URL: d.Event.User.AvatarURL("128")},
		}, auditEntry))
		return err
	}
	return nil
}

func logLeave(d EventData[dg.GuildMemberRemove]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		d.Logger.Debug().Any("event", d.Event).Msg("User Left Guild")
		auditEntry := waitForAuditLog(auditKey{action: dg.AuditLogActionMemberKick, key: d.Event.User.ID})
		if auditEntry != nil {
			_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), addAuditLogFields(&dg.MessageEmbed{
				Title: "User Kicked",
				Color: 0x000088,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
				},
				Thumbnail: &dg.MessageEmbedThumbnail{URL: d.Event.User.AvatarURL("128")},
			}, auditEntry))
			return err
		} else {
			_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Left",
				Color: 0x000088,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
				},
			})
			return err
		}
	}
	return nil
}

func logTimeout(d EventData[dg.GuildMemberUpdate]) error {
	if logChannel, err := config.LogsChannelID.Get(d.Event.GuildID).Value(); err == nil {
		if d.Event.CommunicationDisabledUntil != nil && d.Event.BeforeUpdate != nil && d.Event.CommunicationDisabledUntil != d.Event.BeforeUpdate.CommunicationDisabledUntil {
			// Timeout set
			_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Timed Out",
				Color: 0x888800,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
					{Name: "Until", Value: fmt.Sprintf("<t:%d:f>", d.Event.CommunicationDisabledUntil.Unix()), Inline: true},
				},
			})
			return err
		} else if d.Event.CommunicationDisabledUntil == nil && d.Event.BeforeUpdate != nil && d.Event.BeforeUpdate.CommunicationDisabledUntil != nil {
			// Timeout removed
			_, err := d.Session.ChannelMessageSendEmbed(string(logChannel), &dg.MessageEmbed{
				Title: "User Timeout Removed",
				Color: 0x008888,
				Fields: []*dg.MessageEmbedField{
					{Name: "User", Value: fmt.Sprintf("<@%s>", d.Event.User.ID), Inline: true},
				},
			})
			return err
		}
	}
	return nil
}

func logAuditLog(d EventData[dg.GuildAuditLogEntryCreate]) error {
	switch *d.Event.ActionType {
	case dg.AuditLogActionMessageDelete, dg.AuditLogActionMessageBulkDelete:
		audit.Add(auditKey{action: *d.Event.ActionType, key: d.Event.Options.ChannelID + d.Event.TargetID}, d.Event.AuditLogEntry)
	case dg.AuditLogActionMemberBanAdd, dg.AuditLogActionMemberBanRemove, dg.AuditLogActionMemberKick:
		audit.Add(auditKey{action: *d.Event.ActionType, key: d.Event.TargetID}, d.Event.AuditLogEntry)
	case dg.AuditLogActionMemberUpdate:
		if len(d.Event.Changes) > 0 && *d.Event.Changes[0].Key == dg.AuditLogChangeKeyCommunicationDisabledUntil {
			audit.Add(auditKey{action: *d.Event.ActionType, key: d.Event.TargetID}, d.Event.AuditLogEntry)
		}
	}
	return nil
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
