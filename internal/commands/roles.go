package commands

import (
	"encoding/json"
	"fmt"
	"slices"
	"snoozybot/internal/config"
	"snoozybot/internal/database"
	"snoozybot/internal/i18n"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

var assignTempRole = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name:                     "Assign Temporary Role",
		Type:                     dg.UserApplicationCommand,
		DefaultMemberPermissions: &CommandPermissionModeratorOnly,
	},
	CommandHandler: func(cd *CommandData) error {
		target := cd.ApplicationCommandData().TargetID
		if target == cd.Member.User.ID {
			return cd.Respond(Response{Key: "roles.self"})
		}
		if cd.Member.Permissions&dg.PermissionManageMessages == 0 {
			return cd.Respond(Response{Key: "roles.notMod"})
		}
		tempRoleID, err := config.RolesTempRoleID.Get(cd.GuildID).Value()
		if err != nil || tempRoleID == "" {
			return cd.Respond(Response{Key: "roles.notAvailable"})
		}
		tempRoleIDStr := string(tempRoleID)
		duration, err := config.RolesTempDuration.Get(cd.GuildID).Value()
		if err != nil {
			return cd.Respond(Response{Key: "roles.notAvailable"})
		}
		expires := time.Now().Add(time.Duration(duration) * time.Minute)

		targetMember, err := cd.Session.GuildMember(cd.GuildID, target)
		if err != nil {
			return cd.Respond(Response{Key: "roles.error"})
		}
		if slices.Contains(targetMember.Roles, tempRoleIDStr) {
			return cd.Respond(Response{Key: "roles.alreadyHasRole"})
		}
		cd.Log.Info().Str("guild", cd.GuildID).Str("role", tempRoleIDStr).Str("target", targetMember.User.Username).Msg("Assigning temporary role")
		if err := cd.Session.GuildMemberRoleAdd(cd.GuildID, target, tempRoleIDStr); err != nil {
			cd.Log.Warn().Err(err).Msg("Failed to assign temporary role")
			return cd.Respond(Response{Key: "roles.error"})
		}
		payload, err := json.Marshal(database.ScheduledTaskRemoveRolePayload{RoleID: tempRoleIDStr})
		if err != nil {
			return err
		}
		task := database.ScheduledTask{
			GuildID:      cd.GuildID,
			TaskType:     database.TaskTypeRemoveRole,
			UserID:       target,
			ProcessAfter: expires,
			Payload:      payload,
		}
		if res := database.Database.Create(&task); res.Error != nil {
			return res.Error
		}
		cd.Log.Info().Uint("id", task.ID).Msg("Created role removal task")
		return cd.Respond(Response{Key: "roles.temp.success", Vars: &i18n.Vars{
			"target":  targetMember.User.Mention(),
			"expires": fmt.Sprintf("<t:%d:f>", expires.Unix()),
		}})
	},
}

var assignRegularsRole = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name:                     "Assign Regulars Role",
		Type:                     dg.UserApplicationCommand,
		DefaultMemberPermissions: &CommandPermissionModeratorOnly,
	},
	CommandHandler: func(cd *CommandData) error {
		if cd.Member.Permissions&dg.PermissionManageMessages == 0 {
			return cd.Respond(Response{Key: "roles.notMod"})
		}
		target := cd.ApplicationCommandData().TargetID
		if target == cd.Member.User.ID {
			return cd.Respond(Response{Key: "roles.self"})
		}
		targetMember, err := cd.Session.GuildMember(cd.GuildID, target)
		if err != nil {
			return cd.Respond(Response{Key: "roles.error"})
		}

		return cd.Respond(TryAssignRegularsRole(cd.GuildID, targetMember, &cd.Log, cd.Session))
	},
}

func TryAssignRegularsRole(guildID string, member *dg.Member, logger *zerolog.Logger, session *dg.Session) (response Response) {
	logger.Debug().Str("guild", guildID).Any("member", member).Msg("Attempting to assign regulars role")
	roleID, err := config.RolesRegularsRoleID.Get(guildID).Value()
	if err != nil || roleID == "" {
		return Response{Key: "roles.notAvailable"}
	}
	roleIDStr := string(roleID)

	// At least one of the conditions must exist
	minMessages, _ := config.RolesRegularsMinMessages.Get(guildID).Value()
	minDaysJoined, _ := config.RolesRegularsMinDaysJoined.Get(guildID).Value()
	minDaysActive, _ := config.RolesRegularsMinDaysActive.Get(guildID).Value()
	if minMessages == 0 && minDaysJoined == 0 && minDaysActive == 0 {
		return Response{Key: "roles.notAvailable"}
	}

	if slices.Contains(member.Roles, roleIDStr) {
		return Response{Key: "roles.alreadyHasRole"}
	}
	if minDaysJoined > 0 && member.JoinedAt.Add(time.Duration(minDaysJoined)*24*time.Hour).Before(time.Now()) {
		return Response{Key: "roles.regulars.joinTimeNotMet", Vars: &i18n.Vars{"target": member.Mention()}}
	}

	var metric database.MessageMetric
	if res := database.Database.Where(&database.MessageMetric{
		GuildID: guildID, UserID: member.User.ID,
	}).Take(&metric); res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return Response{Key: "roles.error"}
	}
	if metric.MessageCount < minMessages {
		return Response{Key: "roles.regulars.messageCountNotMet", Vars: &i18n.Vars{"target": member.User.Mention(), "value": minMessages}}
	}
	if metric.DistinctDays < minDaysActive {
		return Response{Key: "roles.regulars.distinctDaysNotMet", Vars: &i18n.Vars{"target": member.User.Mention(), "value": minDaysActive}}
	}

	// All checks have passed, give the role.
	logger.Info().Str("guild", guildID).Str("role", roleIDStr).Str("target", member.User.ID).Msg("Assigning regulars role")
	if err := session.GuildMemberRoleAdd(guildID, member.User.ID, roleIDStr); err != nil {
		logger.Warn().Err(err).Msg("Failed to assign regulars role")
		return Response{Key: "roles.error"}
	}

	// Finally delete the metric row - no longer needed
	database.Database.Delete(&metric)
	return Response{Key: "roles.regulars.success", Vars: &i18n.Vars{"target": member.User.Mention()}}
}
