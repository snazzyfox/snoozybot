package events

var Events = []any{
	createEventHandler("bedtime", bedtimeHandler),
	createEventHandler("memberLeaveCleanup", memberLeaveCleanup),
	createEventHandler("twitchStreamGuildAvailable", twitchStreamGuildAvailable),
	createEventHandler("twitchStreamPresenceUpdate", twitchStreamPresenceUpdate),
	createEventHandler("roleMessageMetricsHandler", roleMessageMetricsHandler),
	createEventHandler("chatMessageCreate", chatMessageCreate),
	createEventHandler("logMessageCreate", logMessageCreate),
	createEventHandler("logMessageUpdate", logMessageUpdate),
	createEventHandler("logMessageDelete", logMessageDelete),
	createEventHandler("logBan", logBan),
	createEventHandler("logUnban", logUnban),
	createEventHandler("logLeave", logLeave),
	createEventHandler("logTimeout", logTimeout),
	createEventHandler("logAuditLog", logAuditLog),
}
