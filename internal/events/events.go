package events

var Handlers = []any{
	bedtimeHandler,
	memberLeaveCleanup,
	twitchStreamGuildAvailable,
	twitchStreamPresenceUpdate,
	roleMessageMetricsHandler,

	logMessageCreate,
	logMessageUpdate,
	logMessageDelete,
	logBan,
	logUnban,
	logLeave,
	logTimeout,
	logAuditLog,
}
