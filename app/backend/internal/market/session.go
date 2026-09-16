package market

import (
	"strings"
	"time"

	"trading-agents/internal/dataflows"
)

func normalizedExchangeSession(symbol, upstream string, now time.Time) string {
	upstream = strings.ToLower(strings.TrimSpace(upstream))
	if upstream == "closed" || upstream == "market closed" {
		return "closed"
	}
	return exchangeSessionAt(symbol, now)
}

func exchangeSessionAt(symbol string, now time.Time) string {
	switch dataflows.DetectMarket(symbol) {
	case dataflows.MarketCrypto:
		return "regular"
	case dataflows.MarketUS:
		return usSessionAt(now)
	case dataflows.MarketCN:
		return splitSessionAt(now, "Asia/Shanghai", 9*60+30, 11*60+30, 13*60, 15*60, cnExchangeHoliday)
	case dataflows.MarketHK:
		return hkSessionAt(now)
	default:
		return "closed"
	}
}

func hkSessionAt(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		return "closed"
	}
	local := now.In(loc)
	if !isWeekday(local) {
		return "closed"
	}
	if closed, covered := hkExchangeHoliday(local); !covered || closed {
		return "closed"
	}
	minute := local.Hour()*60 + local.Minute()
	halfDays := map[string]bool{"2026-02-16": true, "2026-12-24": true, "2026-12-31": true}
	if halfDays[local.Format("2006-01-02")] {
		if minute >= 9*60+30 && minute < 12*60 {
			return "regular"
		}
		return "closed"
	}
	if (minute >= 9*60+30 && minute < 12*60) || (minute >= 13*60 && minute < 16*60) {
		return "regular"
	}
	return "closed"
}

func usSessionAt(now time.Time) string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return "closed"
	}
	local := now.In(loc)
	if !isWeekday(local) || isUSExchangeHoliday(local) {
		return "closed"
	}
	minute := local.Hour()*60 + local.Minute()
	switch {
	case minute >= 4*60 && minute < 9*60+30:
		return "pre"
	case minute >= 9*60+30 && minute < 16*60:
		return "regular"
	case minute >= 16*60 && minute < 20*60:
		return "post"
	default:
		return "closed"
	}
}

func splitSessionAt(now time.Time, zone string, open1, close1, open2, close2 int, holiday func(time.Time) (bool, bool)) string {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "closed"
	}
	local := now.In(loc)
	if !isWeekday(local) {
		return "closed"
	}
	if closed, covered := holiday(local); !covered || closed {
		return "closed"
	}
	minute := local.Hour()*60 + local.Minute()
	if (minute >= open1 && minute < close1) || (minute >= open2 && minute < close2) {
		return "regular"
	}
	return "closed"
}

func cnExchangeHoliday(value time.Time) (bool, bool) {
	if value.Year() != 2026 {
		return false, false
	}
	return dateInRanges(value, [][2]string{
		{"2026-01-01", "2026-01-03"}, {"2026-02-15", "2026-02-23"},
		{"2026-04-04", "2026-04-06"}, {"2026-05-01", "2026-05-05"},
		{"2026-06-19", "2026-06-21"}, {"2026-09-25", "2026-09-27"},
		{"2026-10-01", "2026-10-07"},
	}), true
}

func hkExchangeHoliday(value time.Time) (bool, bool) {
	if value.Year() != 2026 {
		return false, false
	}
	closed := map[string]bool{
		"2026-01-01": true, "2026-02-17": true, "2026-02-18": true, "2026-02-19": true,
		"2026-04-03": true, "2026-04-06": true, "2026-04-07": true, "2026-05-01": true,
		"2026-05-25": true, "2026-06-19": true, "2026-07-01": true, "2026-10-01": true,
		"2026-10-19": true, "2026-12-25": true,
	}
	return closed[value.Format("2006-01-02")], true
}

func dateInRanges(value time.Time, ranges [][2]string) bool {
	date := value.Format("2006-01-02")
	for _, item := range ranges {
		if date >= item[0] && date <= item[1] {
			return true
		}
	}
	return false
}

func isWeekday(value time.Time) bool {
	return value.Weekday() != time.Saturday && value.Weekday() != time.Sunday
}

func isUSExchangeHoliday(value time.Time) bool {
	year := value.Year()
	date := localDate(value)
	holidays := []time.Time{
		observedDate(time.Date(year, time.January, 1, 0, 0, 0, 0, value.Location())),
		nthWeekday(year, time.January, time.Monday, 3, value.Location()),
		nthWeekday(year, time.February, time.Monday, 3, value.Location()),
		easterSunday(year, value.Location()).AddDate(0, 0, -2),
		lastWeekday(year, time.May, time.Monday, value.Location()),
		observedDate(time.Date(year, time.June, 19, 0, 0, 0, 0, value.Location())),
		observedDate(time.Date(year, time.July, 4, 0, 0, 0, 0, value.Location())),
		nthWeekday(year, time.September, time.Monday, 1, value.Location()),
		nthWeekday(year, time.November, time.Thursday, 4, value.Location()),
		observedDate(time.Date(year, time.December, 25, 0, 0, 0, 0, value.Location())),
	}
	for _, holiday := range holidays {
		if date.Equal(localDate(holiday)) {
			return true
		}
	}
	return false
}

func localDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func observedDate(value time.Time) time.Time {
	if value.Weekday() == time.Saturday {
		return value.AddDate(0, 0, -1)
	}
	if value.Weekday() == time.Sunday {
		return value.AddDate(0, 0, 1)
	}
	return value
}

func nthWeekday(year int, month time.Month, weekday time.Weekday, n int, loc *time.Location) time.Time {
	date := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	offset := (int(weekday) - int(date.Weekday()) + 7) % 7
	return date.AddDate(0, 0, offset+7*(n-1))
}

func lastWeekday(year int, month time.Month, weekday time.Weekday, loc *time.Location) time.Time {
	date := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
	offset := (int(date.Weekday()) - int(weekday) + 7) % 7
	return date.AddDate(0, 0, -offset)
}

func easterSunday(year int, loc *time.Location) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
}
