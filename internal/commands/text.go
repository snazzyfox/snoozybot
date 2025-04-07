package commands

import (
	"fmt"
	"math/rand"
	"snoozybot/internal/cooldown"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
)

var flip = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "flip",
	},
	CommandHandler: func(cd *CommandData) error {
		// 1% chance of the flip failing
		if rand.Float32() < 0.01 {
			return cd.Respond(Response{Key: "flip.failure", Public: true})
		}

		result := i18n.Get(*cd.Interaction.GuildLocale, lo.Ternary(rand.Float32() < 0.5, "flip.heads", "flip.tails"))
		return cd.Respond(Response{Key: "flip.success", Vars: &i18n.Vars{"result": result}, Public: true})
	},
}

var roll = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "roll",
		Options: []*dg.ApplicationCommandOption{
			{Type: dg.ApplicationCommandOptionInteger, Name: "sides", Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		sides := cd.Option("sides").IntValue()
		if sides < 3 {
			return cd.Respond(Response{Key: "roll.tooFewSides", Vars: &i18n.Vars{"sides": sides}, Public: true})
		}
		if sides > 120 {
			return cd.Respond(Response{Key: "roll.tooManySides", Vars: &i18n.Vars{"sides": sides}, Public: true})
		}
		result := rand.Int63n(sides) + 1
		return cd.Respond(Response{Key: "roll.success", Vars: &i18n.Vars{"sides": sides, "result": result}, Public: true})
	},
}

func createTargetedCommand(name string) *BotCommand {
	return &BotCommand{
		ApplicationCommand: dg.ApplicationCommand{
			Name: name,
			Options: []*dg.ApplicationCommandOption{
				{Type: dg.ApplicationCommandOptionUser, Name: "user", Required: true},
			},
		},
		CommandHandler: handleTargetedCommand,
	}
}

var cdm = cooldown.Initialize(cooldown.CooldownManager{
	UserInvocations:    3,
	ChannelInvocations: 5,
	CooldownDuration:   time.Minute * 5,
})

func handleTargetedCommand(cd *CommandData) error {
	// Check cooldown first
	if !cdm.Can(cd.GuildID, cd.ChannelID, cd.Interaction.Member) {
		return cd.Respond(Response{Key: "base.cooldown"})
	}

	user := cd.Option("user").UserValue(cd.Session)
	if user == nil {
		return fmt.Errorf("user not found")
	}
	command := cd.ApplicationCommandData().Name
	vars := i18n.GetFragments(*cd.Interaction.GuildLocale, command)
	vars["author"] = cd.Interaction.Member.DisplayName()

	var key string
	if user.ID == cd.Interaction.Member.User.ID {
		vars["target"] = cd.Interaction.Member.DisplayName()
		key = command + ".self"
	} else if user.ID == cd.State.User.ID {
		// The bot's name is supposed to be in GlobalName by discord docs but discord is broken
		botMember, _ := cd.State.Member(cd.GuildID, cd.State.User.ID)
		vars["target"] = lo.CoalesceOrEmpty(botMember.DisplayName(), cd.State.Application.Name, cd.State.User.Username)
		key = command + ".bot"
	} else {
		vars["target"] = mentionIfWanted(lo.Must(cd.GuildMember(cd.GuildID, user.ID)))
		key = command + ".user"
	}
	cd.Log.Debug().Str("key", key).Any("vars", vars).Msg("Responding to targeted command")
	return cd.Respond(Response{Key: key, Vars: &vars, Public: true, InteractionResponseData: dg.InteractionResponseData{
		AllowedMentions: &dg.MessageAllowedMentions{
			Parse: []dg.AllowedMentionType{dg.AllowedMentionTypeUsers},
		},
	}})
}

func mentionIfWanted(target *dg.Member) string {
	user, err := database.GetUser(target.User.ID)
	if err != nil || user.SuppressMentions || target.Permissions&dg.PermissionManageMessages != 0 {
		return target.DisplayName()
	}
	return target.Mention()
}
