package events

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"snoozybot/internal/config"
	"snoozybot/internal/cooldown"
	"snoozybot/internal/i18n"
	"strings"
	"text/template"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

//go:embed chat.defaultprompt.txt
var defaultPrompt string

var client = openai.NewClient(option.WithAPIKey(lo.Must(os.LookupEnv("OPENAI_API_KEY"))))

var cdm = cooldown.Initialize(cooldown.CooldownManager{
	UserInvocations:    2,
	ChannelInvocations: 5,
	CooldownDuration:   time.Minute * 3,
})

const CACHE_CAPACITY = 10

/* RollingCache */
type rollingCache[T any] struct {
	items   []T
	current int
}

func (c *rollingCache[T]) Add(item T) {
	c.items[c.current] = item
	c.current = (c.current + 1) % len(c.items)
}

func (c *rollingCache[T]) GetAll() []T {
	return append(c.items[c.current:], c.items[:c.current]...)
}

var chatCaches = map[string]*rollingCache[*dg.Message]{}

func addMessage(channelID string, message *dg.Message) {
	cache, ok := chatCaches[channelID]
	if !ok {
		cache = &rollingCache[*dg.Message]{
			items:   make([]*dg.Message, CACHE_CAPACITY),
			current: 0,
		}
		chatCaches[channelID] = cache
	}
	cache.Add(message)
}

func chatMessageCreate(d EventData[dg.MessageCreate]) error {
	roleIDs, err := config.ChatRoleIDs.Get(d.Event.GuildID).Value()
	if err != nil {
		// no role IDs means chat functionality not available. To enable for everyone, add the role ID for @everyone in that server.
		d.Logger.Trace().Str("guild_id", d.Event.GuildID).Msg("AI Chat not configured for this server.")
		return nil
	}

	// Chat functionality is enabled. Add the message to cache here.
	d.Logger.Trace().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Str("user_id", d.Event.Author.ID).Msg("Adding message to AI chat cache.")
	addMessage(d.Event.ChannelID, d.Event.Message)

	if d.Event.Author.Bot ||
		d.Event.Mentions == nil ||
		!lo.ContainsBy(d.Event.Mentions, func(mention *dg.User) bool { return mention.ID == d.Session.State.User.ID }) {
		return nil
	}

	d.Logger.Trace().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Str("user_id", d.Event.Author.ID).Msg("User mentioned bot. Attempting AI response.")
	d.Event.Member.User = d.Event.Author
	if !cdm.Can(d.Event.GuildID, d.Event.ChannelID, d.Event.Member) {
		guild, _ := d.Session.Guild(d.Event.GuildID)
		d.Session.ChannelMessageSendReply(d.Event.ChannelID, i18n.Get(dg.Locale(guild.PreferredLocale), "chat.cooldown"), &dg.MessageReference{
			MessageID: d.Event.Message.ID,
			ChannelID: d.Event.ChannelID,
			GuildID:   d.Event.GuildID,
		})
		return nil
	}
	var prompt string
	var history []*dg.Message
	if len(roleIDs) > 0 && lo.None(lo.Map(roleIDs, func(id json.Number, _ int) string { return string(id) }), d.Event.Member.Roles) {
		// user doesn't have chat role. Use predefined prompt.
		d.Logger.Debug().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Str("user_id", d.Event.Author.ID).Msg("User does not have chat role. Using default prompt.")
		prompt = defaultPrompt
		history = []*dg.Message{d.Event.Message}
	} else {
		d.Logger.Debug().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Str("user_id", d.Event.Author.ID).Msg("User has chat role. Using custom prompt.")
		promptLines, err := config.ChatPrompts.Get(d.Event.GuildID).Value()
		if err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Err(err).Msg("Chat role IDs set, but failed to get chat prompt")
		}
		tmpl, err := template.New("prompt").Parse(strings.Join(promptLines, "\n"))
		if err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Strs("prompt", promptLines).Err(err).Msg("Failed to parse chat prompt")
		}
		member, err := d.Session.State.Member(d.Event.GuildID, d.Session.State.User.ID)
		if err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Err(err).Msg("Failed to get bot member")
		}
		guild, err := d.Session.State.Guild(d.Event.GuildID)
		if err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Err(err).Msg("Failed to get guild")
		}
		prompt = i18n.TemplateString(tmpl, &i18n.Vars{
			"bot_name":         member.DisplayName(),
			"guild_name":       guild.Name,
			"current_time_utc": time.Now().UTC().Format(time.RFC1123),
			"user_name":        d.Event.Member.DisplayName(),
			"user_mention":     d.Event.Member.Mention(),
			"is_mod":           d.Event.Member.Permissions&dg.PermissionManageMessages != 0,
		})
		history = chatCaches[d.Event.ChannelID].GetAll()
	}
	response, err := getAIResponse(prompt, history, d.Logger)
	if err != nil {
		return err
	}
	d.Session.ChannelMessageSendReply(d.Event.ChannelID, response, &dg.MessageReference{
		MessageID: d.Event.Message.ID,
		ChannelID: d.Event.ChannelID,
		GuildID:   d.Event.GuildID,
	})
	return nil
}

func getAIResponse(prompt string, history []*dg.Message, log *zerolog.Logger) (string, error) {
	prompt += "\n\nThe message contains the most recent conversations in the channel for context. Respond only to the last message."
	messagePrompt := strings.Join(lo.Map(history, func(message *dg.Message, _ int) string {
		if message != nil {
			return fmt.Sprintf("[%s] %s", message.Author.Username, message.Content)
		}
		return ""
	}), "\n")

	log.Debug().Str("system", prompt).Str("message", messagePrompt).Msg("Sending message to AI.")

	resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Model:           openai.ChatModelGPT4oMini,
		Instructions:    openai.String(prompt),
		Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String(messagePrompt)},
		Temperature:     openai.Float(1.2),
		MaxOutputTokens: openai.Int(300),
	})
	if err != nil {
		return "", err
	}
	return resp.OutputText(), nil
}
