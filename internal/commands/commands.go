package commands

/** The actual command list available to the app */
var Commands = []*BotCommand{
	&echo,
	&petpet,
	&report,
	&my,
	&quote,
	&quotes,
	&quotesAddContextMenu,
	&reminder,
	&assignTempRole,
	&assignRegularsRole,
	&admin,

	&flip,
	&roll,
	createTargetedCommand("bap"),
	createTargetedCommand("boop"),
	createTargetedCommand("bonk"),
	createTargetedCommand("cute"),
	createTargetedCommand("hug"),
	createTargetedCommand("pet"),
	createTargetedCommand("tuck"),
	createTargetedCommand("pour"),
}
