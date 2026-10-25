package statistics

import "time"

// Days in the range when none are given
const defaultRangeDays = 30

// rangeStart is the start of the last number of days, including today, and what each point of its graphs is:
// a day up to 45 days, a week up to about a year, and a month for longer
func rangeStart(days int, now time.Time) (since time.Time, period string) {
	period = "day"
	switch {
	case days > 400:
		period = "month"
	case days > 45:
		period = "week"
	}
	return periodStart(startOfDay(now).AddDate(0, 0, -(days-1)), period), period
}

func startOfDay(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// periodStart is the first day of the day, week (from Monday) or month, the same as the database groups them
func periodStart(t time.Time, period string) time.Time {
	day := startOfDay(t)
	switch period {
	case "week":
		return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
	default:
		return day
	}
}

func nextPeriod(start time.Time, period string) time.Time {
	switch period {
	case "week":
		return start.AddDate(0, 0, 7)
	case "month":
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}

// periodStarts is the first day of each day, week or month from since to now
func periodStarts(since, now time.Time, period string) []time.Time {
	var starts []time.Time
	for start := periodStart(since, period); !start.After(now); start = nextPeriod(start, period) {
		starts = append(starts, start)
	}
	return starts
}
