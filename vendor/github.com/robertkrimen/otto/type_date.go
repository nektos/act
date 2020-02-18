package otto

import (
	"fmt"
	"math"
	"regexp"
	Time "time"
)

type _dateObject struct {
	time  Time.Time // Time from the "time" package, a cached version of time
	epoch int64
	value Value
	isNaN bool
}

var (
	invalidDateObject = _dateObject{
		time:  Time.Time{},
		epoch: -1,
		value: NaNValue(),
		isNaN: true,
	}
)

type _ecmaTime struct {
	year        int
	month       int
	day         int
	hour        int
	minute      int
	second      int
	millisecond int
	location    *Time.Location // Basically, either local or UTC
}

func ecmaTime(goTime Time.Time) _ecmaTime {
	return _ecmaTime{
		goTime.Year(),
		dateFromGoMonth(goTime.Month()),
		goTime.Day(),
		goTime.Hour(),
		goTime.Minute(),
		goTime.Second(),
		goTime.Nanosecond() / (100 * 100 * 100),
		goTime.Location(),
	}
}

func (self *_ecmaTime) goTime() Time.Time {
	return Time.Date(
		self.year,
		dateToGoMonth(self.month),
		self.day,
		self.hour,
		self.minute,
		self.second,
		self.millisecond*(100*100*100),
		self.location,
	)
}

func (self *_dateObject) Time() Time.Time {
	return self.time
}

func (self *_dateObject) Epoch() int64 {
	return self.epoch
}

func (self *_dateObject) Value() Value {
	return self.value
}

// FIXME A date should only be in the range of -100,000,000 to +100,000,000 (1970): 15.9.1.1
func (self *_dateObject) SetNaN() {
	self.time = Time.Time{}
	self.epoch = -1
	self.value = NaNValue()
	self.isNaN = true
}

func (self *_dateObject) SetTime(time Time.Time) {
	self.Set(timeToEpoch(time))
}

func epoch2dateObject(epoch float64) _dateObject {
	date := _dateObject{}
	date.Set(epoch)
	return date
}

func (self *_dateObject) Set(epoch float64) {
	// epoch
	self.epoch = epochToInteger(epoch)

	// time
	time, err := epochToTime(epoch)
	self.time = time // Is either a valid time, or the zero-value for time.Time

	// value & isNaN
	if err != nil {
		self.isNaN = true
		self.epoch = -1
		self.value = NaNValue()
	} else {
		self.value = toValue_int64(self.epoch)
	}
}

func epochToInteger(value float64) int64 {
	if value > 0 {
		return int64(math.Floor(value))
	}
	return int64(math.Ceil(value))
}

func epochToTime(value float64) (time Time.Time, err error) {
	epochWithMilli := value
	if math.IsNaN(epochWithMilli) || math.IsInf(epochWithMilli, 0) {
		err = fmt.Errorf("Invalid time %v", value)
		return
	}

	epoch := int64(epochWithMilli / 1000)
	milli := int64(epochWithMilli) % 1000

	time = Time.Unix(int64(epoch), milli*1000000).UTC()
	return
}

func timeToEpoch(time Time.Time) float64 {
	return float64(time.UnixNano() / (1000 * 1000))
}

func (runtime *_runtime) newDateObject(epoch float64) *_object {
	self := runtime.newObject()
	self.class = "Date"

	// FIXME This is ugly...
	date := _dateObject{}
	date.Set(epoch)
	self.value = date
	return self
}

func (self *_object) dateValue() _dateObject {
	value, _ := self.value.(_dateObject)
	return value
}

func dateObjectOf(rt *_runtime, _dateObject *_object) _dateObject {
	if _dateObject == nil || _dateObject.class != "Date" {
		panic(rt.panicTypeError())
	}
	return _dateObject.dateValue()
}

// JavaScript is 0-based, Go is 1-based (15.9.1.4)
func dateToGoMonth(month int) Time.Month {
	return Time.Month(month + 1)
}

func dateFromGoMonth(month Time.Month) int {
	return int(month) - 1
}

// Both JavaScript & Go are 0-based (Sunday == 0)
func dateToGoDay(day int) Time.Weekday {
	return Time.Weekday(day)
}

func dateFromGoDay(day Time.Weekday) int {
	return int(day)
}

func newDateTime(argumentList []Value, location *Time.Location) (epoch float64) {

	pick := func(index int, default_ float64) (float64, bool) {
		if index >= len(argumentList) {
			return default_, false
		}
		value := argumentList[index].float64()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, true
		}
		return value, false
	}

	if len(argumentList) >= 2 { // 2-argument, 3-argument, ...
		var year, month, day, hour, minute, second, millisecond float64
		var invalid bool
		if year, invalid = pick(0, 1900.0); invalid {
			goto INVALID
		}
		if month, invalid = pick(1, 0.0); invalid {
			goto INVALID
		}
		if day, invalid = pick(2, 1.0); invalid {
			goto INVALID
		}
		if hour, invalid = pick(3, 0.0); invalid {
			goto INVALID
		}
		if minute, invalid = pick(4, 0.0); invalid {
			goto INVALID
		}
		if second, invalid = pick(5, 0.0); invalid {
			goto INVALID
		}
		if millisecond, invalid = pick(6, 0.0); invalid {
			goto INVALID
		}

		if year >= 0 && year <= 99 {
			year += 1900
		}

		time := Time.Date(int(year), dateToGoMonth(int(month)), int(day), int(hour), int(minute), int(second), int(millisecond)*1000*1000, location)
		return timeToEpoch(time)

	} else if len(argumentList) == 0 { // 0-argument
		time := Time.Now().UTC()
		return timeToEpoch(time)
	} else { // 1-argument
		value := valueOfArrayIndex(argumentList, 0)
		value = toPrimitive(value)
		if value.IsString() {
			return dateParse(value.string())
		}

		return value.float64()
	}

INVALID:
	epoch = math.NaN()
	return
}

var (
	dateLayoutList = []string{
		"2006",
		"2006-01",
		"2006-01-02",

		"2006T15:04",
		"2006-01T15:04",
		"2006-01-02T15:04",

		"2006T15:04:05",
		"2006-01T15:04:05",
		"2006-01-02T15:04:05",

		"2006T15:04:05.000",
		"2006-01T15:04:05.000",
		"2006-01-02T15:04:05.000",

		"2006T15:04-0700",
		"2006-01T15:04-0700",
		"2006-01-02T15:04-0700",

		"2006T15:04:05-0700",
		"2006-01T15:04:05-0700",
		"2006-01-02T15:04:05-0700",

		"2006T15:04:05.000-0700",
		"2006-01T15:04:05.000-0700",
		"2006-01-02T15:04:05.000-0700",

		Time.RFC1123,
	}
	matchDateTimeZone = regexp.MustCompile(`^(.*)(?:(Z)|([\+\-]\d{2}):(\d{2}))$`)
)

func dateParse(date string) (epoch float64) {
	// YYYY-MM-DDTHH:mm:ss.sssZ
	var time Time.Time
	var err error
	{
		date := date
		if match := matchDateTimeZone.FindStringSubmatch(date); match != nil {
			if match[2] == "Z" {
				date = match[1] + "+0000"
			} else {
				date = match[1] + match[3] + match[4]
			}
		}
		for _, layout := range dateLayoutList {
			time, err = Time.Parse(layout, date)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return math.NaN()
	}
	return float64(time.UnixNano()) / (1000 * 1000) // UnixMilli()
}
