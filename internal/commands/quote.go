package commands

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

var quote = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "quote",
		Options: []*dg.ApplicationCommandOption{
			{Name: "user", Type: dg.ApplicationCommandOptionUser, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		target, err := cd.GuildMember(cd.GuildID, cd.Option("user").UserValue(cd.Session).ID)
		if err != nil {
			return err
		}
		var quote database.Quote
		if res := database.Database.Where(&database.Quote{GuildID: cd.GuildID, UserID: target.User.ID}).Order("random()").Select("content").First(&quote); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return cd.Respond(Response{Key: "quote.missing"})
			}
			return res.Error
		}
		return cd.Respond(Response{
			Key:    "quote.success",
			Vars:   &i18n.Vars{"user": target.DisplayName(), "content": quote.Content},
			Public: true,
		})
	},
}

var quotes = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{Name: "quotes", DefaultMemberPermissions: &CommandPermissionModeratorOnly},
	Subcommands: []*BotCommand{
		&quotesGet,
		&quotesAdd,
		&quotesFind,
		&quotesDelete,
	},
}

var quotesGet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "get",
		Options: []*dg.ApplicationCommandOption{
			{Name: "id", Type: dg.ApplicationCommandOptionInteger, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		id := cd.Option("id").UintValue()
		quote := database.Quote{ID: uint(id)}
		if res := database.Database.Take(&quote); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return cd.Respond(Response{Key: "quote.missing"})
			}
			return res.Error
		}
		target, err := cd.GuildMember(cd.GuildID, quote.UserID)
		if err != nil {
			return err
		}
		return cd.Respond(Response{
			Key:    "quote.success",
			Vars:   &i18n.Vars{"user": target.DisplayName(), "content": quote.Content},
			Public: true,
		})
	},
}

var quotesAdd = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "add",
		Options: []*dg.ApplicationCommandOption{
			{Name: "user", Type: dg.ApplicationCommandOptionUser, Required: true},
			{Name: "text", Type: dg.ApplicationCommandOptionString, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		user := cd.Option("user").UserValue(cd.Session)
		content := cd.Option("text").StringValue()
		return _addQuote(cd, user, content)
	},
}

var quotesAddContextMenu = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "Add Quote",
		Type: dg.MessageApplicationCommand,
	},
	CommandHandler: func(cd *CommandData) error {
		message := cd.ApplicationCommandData().Resolved.Messages[cd.ApplicationCommandData().TargetID]
		return _addQuote(cd, message.Author, message.Content)
	},
}

func _addQuote(cd *CommandData, user *dg.User, content string) error {
	if user.ID == cd.State.User.ID {
		return cd.Respond(Response{Key: "quotes/add.botTarget"})
	}
	digest := _computeDigest(content)
	quote := database.Quote{GuildID: cd.GuildID, UserID: user.ID, AddedBy: cd.Member.User.ID, Content: content, ContentDigest: digest}
	if result := database.Database.Create(&quote); result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return cd.Respond(Response{Key: "quotes/add.duplicate"})
		}
		return result.Error
	}
	cd.Log.Info().Uint("quote", quote.ID).Msg("Created quote.")
	return cd.Respond(Response{Key: "quotes/add.success", Vars: &i18n.Vars{"id": quote.ID}})
}

func _computeDigest(s string) string {
	// Remove all nonalphanumeric so similar variations with only spacing/casing differences are easily deduped
	bytes := []byte(s)
	i := 0
	for _, b := range bytes {
		if ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') {
			bytes[i] = b
			i++
		}
	}
	bytes = bytes[:i]
	// Actually compute the hash
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:])
}

var quotesDelete = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "delete",
		Options: []*dg.ApplicationCommandOption{
			{Name: "id", Type: dg.ApplicationCommandOptionInteger, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		id := cd.Option("id").UintValue()
		quote := database.Quote{ID: uint(id)}
		if res := database.Database.Delete(&quote); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return cd.Respond(Response{Key: "quotes/delete.missing"})
			}
			return res.Error
		}
		cd.Log.Info().Uint("quote", quote.ID).Msg("Deleted quote.")
		return cd.Respond(Response{Key: "quotes/delete.success"})
	},
}

var quotesFind = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "find",
		Options: []*dg.ApplicationCommandOption{
			{Name: "text_search", Type: dg.ApplicationCommandOptionString},
			{Name: "user", Type: dg.ApplicationCommandOptionUser},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		text, _ := lo.TryOr(func() (string, error) {
			return cd.Option("text_search").StringValue(), nil
		}, "")
		user, _ := lo.TryOr(func() (*dg.User, error) {
			return cd.Option("user").UserValue(cd.Session), nil
		}, nil)
		var quotes []*database.Quote
		query := database.Database.Limit(11)
		if user != nil {
			query = query.Where(&database.Quote{UserID: user.ID})
		}
		if text != "" {
			query = query.Where("content LIKE ?", fmt.Sprintf("%%%s%%", text))
		}
		if res := query.Find(&quotes); res.Error != nil {
			return res.Error
		} else if res.RowsAffected == 0 {
			return cd.Respond(Response{Key: "quotes/find.empty"})
		}
		embed := &dg.MessageEmbed{
			Fields: lo.Map(lo.Slice(quotes, 0, 10), func(q *database.Quote, _ int) *dg.MessageEmbedField {
				var userName string
				if user, err := cd.GuildMember(cd.GuildID, q.UserID); err == nil {
					userName = user.DisplayName()
				} else {
					cd.Log.Warn().Str("user", q.UserID).Err(err).Msg("Failed to get user for quote")
					userName = "Unknown User"
				}
				return &dg.MessageEmbedField{
					Name:  fmt.Sprintf("[#%d] %s", q.ID, userName),
					Value: q.Content,
				}
			}),
		}
		if len(quotes) > 10 {
			embed.Footer = &dg.MessageEmbedFooter{Text: i18n.Get(*cd.GuildLocale, "responses.quotes/find.hasMore")}
		}
		return cd.InteractionRespond(cd.Interaction, &dg.InteractionResponse{
			Type: dg.InteractionResponseChannelMessageWithSource,
			Data: &dg.InteractionResponseData{
				Embeds: []*dg.MessageEmbed{embed},
				Flags:  dg.MessageFlagsEphemeral,
			},
		})
	},
}
