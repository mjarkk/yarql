package helpers

import (
	"errors"
	"time"
)

// The ISO 8601 layout might also be "2006-01-02T15:04:05.999Z" but it's mentioned less than the current so i presume what we're now using is correct
var timeISO8601Layout = "2006-01-02T15:04:05.000Z"

func ParseIso8601String(val string) (time.Time, error) {
	parsedTime, err := time.Parse(timeISO8601Layout, val)
	if err != nil {
		return time.Time{}, errors.New("time value doesn't match the ISO 8601 layout")
	}
	return parsedTime, nil
}

func TimeToIso8601String(target *[]byte, t time.Time) {
	*target = t.AppendFormat(*target, timeISO8601Layout)
}
