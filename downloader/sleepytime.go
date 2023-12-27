package downloader

type sleepytime struct {
	next_sleep int
	max_sleep  int
	half_sleep int
}

func (sleeper *sleepytime) reset(initial_time int, max_time int) {
	sleeper.next_sleep = initial_time
	sleeper.max_sleep = max_time
	sleeper.half_sleep = max_time / 2
}

func (sleeper *sleepytime) getNextSleep() int {
	if sleeper.next_sleep < sleeper.half_sleep {
		sleeper.next_sleep = sleeper.next_sleep * 2
	} else {
		sleeper.next_sleep = sleeper.max_sleep
	}
	return sleeper.next_sleep
}
