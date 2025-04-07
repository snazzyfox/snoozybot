package tasks

var Tasks = []*PeriodicTask{
	&storedScheduledTask,
	&youtubeNotificationTask,
	&bskyNotificationTask,
}
