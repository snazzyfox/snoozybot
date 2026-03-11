package events

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"snoozybot/internal/config"
	"strings"

	dg "github.com/bwmarrin/discordgo"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
)

//go:embed automod.header.txt
var headerPrompt string

type automodResponse struct {
	Result string `json:"result" jsonschema:"enum=pass,enum=fail"`
	Reason string `json:"reason"`
}

func generateSchema[T any]() map[string]any {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)

	data, _ := json.Marshal(schema)
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}

var automodResponseSchema = generateSchema[automodResponse]()

func automodMessageCreate(d EventData[dg.MessageCreate]) error {
	// Ignore messages that don't have text.
	if len(d.Event.Content) == 0 {
		return nil
	}

	rules, err := config.AutomodRulePrompts.Get(d.Event.GuildID).Value()
	if err != nil {
		// no role IDs means chat functionality not available. To enable for everyone, add the role ID for @everyone in that server.
		d.Logger.Trace().Str("guild_id", d.Event.GuildID).Msg("AI Automod not enabled for this server.")
		return nil
	}

	// AI Automod is enabled.
	// Determine if it's a channel we want to scan
	includeChannelIDs, err := config.AutomodIncludeChannelIDs.Get(d.Event.GuildID).Value() // ignore error; ok to leave out
	excludeChannelIDs, err := config.AutomodIgnoreChannelIDs.Get(d.Event.GuildID).Value()
	if (includeChannelIDs != nil && !slices.Contains(includeChannelIDs, json.Number(d.Event.ChannelID))) ||
		(excludeChannelIDs != nil && slices.Contains(excludeChannelIDs, json.Number(d.Event.ChannelID))) {
		d.Logger.Trace().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Msg("Channel excluded from AI Automod.")
		return nil
	}

	// Send the message to AI for evaluation.
	response, err := automodGetAIResponse(d, rules)
	if err != nil {
		return err
	}
	switch response.Result {
	case "pass":
		d.Logger.Debug().Any("response", response).Msg("Got passing response. Continuing without action.")
	case "fail":
		d.Logger.Info().Any("response", response).Msg("Got failed AI automod response. Deleting message.")
		if err := d.Session.ChannelMessageDelete(
			d.Event.ChannelID, d.Event.Message.ID, dg.WithAuditLogReason("Failed AI Automod with reason: "+response.Reason),
		); err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Str("channel_id", d.Event.ChannelID).Str("message_id", d.Event.Message.ID).Err(err).Msg("Failed to delete message from Discord.")
			return err
		}

		logChannel, err := config.AutomodLogChannelID.Get(d.Event.GuildID).Value()
		if err != nil {
			d.Logger.Warn().Str("guild_id", d.Event.GuildID).Msg("AI Automod log channel not configured; skipping notification.")
			return nil
		}
		if _, err := d.Session.ChannelMessageSendEmbed(logChannel.String(), &dg.MessageEmbed{
			Title: "Message Blocked by Automod",
			Color: 0xE63946,
			Fields: []*dg.MessageEmbedField{
				{Name: "Channel", Value: fmt.Sprintf("<#%s>", d.Event.ChannelID), Inline: true},
				{Name: "Author", Value: fmt.Sprintf("<@%s>", d.Event.Message.Author.ID), Inline: true},
				{Name: "Sent at", Value: fmt.Sprintf("<t:%d:f>", d.Event.Message.Timestamp.Unix()), Inline: true},
				{Name: "Message URL", Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", d.Event.GuildID, d.Event.ChannelID, d.Event.Message.ID)},
				{Name: "Reason", Value: response.Reason},
				{Name: "Content", Value: d.Event.Message.Content},
			},
		}); err != nil {
			d.Logger.Error().Str("guild_id", d.Event.GuildID).Str("channel_id", logChannel.String()).Err(err).Msg("Failed to send mod alert to AI Automod log channel.")
			return err
		}

	default:
		d.Logger.Error().Any("response", response).Msg("Got unexpected AI automod response. Skipping processing.")
	}
	return nil
}

func automodGetAIResponse(d EventData[dg.MessageCreate], rulePrompts []string) (*automodResponse, error) {
	// Send the message to AI for evaluation.
	prompt := headerPrompt + "\n" + strings.Join(rulePrompts, "\n")
	d.Logger.Debug().Str("system", prompt).Str("message", d.Event.Content).Msg("Sending message to AI Automod.")
	resp, err := client.Responses.New(context.Background(), responses.ResponseNewParams{
		Model:           openai.ChatModelGPT5Mini,
		Instructions:    openai.String(prompt),
		Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String(d.Event.Content)},
		MaxOutputTokens: openai.Int(300),
		Text: responses.ResponseTextConfigParam{
			Format:    responses.ResponseFormatTextConfigParamOfJSONSchema("automod_response", automodResponseSchema),
			Verbosity: responses.ResponseTextConfigVerbosityLow,
		},
		Reasoning: shared.ReasoningParam{Effort: openai.ReasoningEffortLow},
	})
	if err != nil {
		return nil, err
	}

	// Decode the response
	var response automodResponse
	if err := json.Unmarshal([]byte(resp.OutputText()), &response); err != nil {
		return nil, err
	}
	return &response, nil
}
