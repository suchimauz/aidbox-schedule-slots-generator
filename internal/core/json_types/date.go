package json_types

import (
	"encoding/json"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/utils"
)

type DateTime struct {
	Date time.Time
}

func (t *DateTime) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := utils.ParseDate(str)
	if err != nil {
		return err
	}

	*t = DateTime{Date: parsedDate}
	return nil
}

func (t DateTime) MarshalJSON() ([]byte, error) {
	// Устанавливаем временную зону
	location := config.TimeZone
	dateInLocation := t.Date.In(location)
	return json.Marshal(dateInLocation.Format("2006-01-02T15:04:05-07:00"))
}

type DateTimeWithoutTimezone struct {
	Date time.Time
}

func (t *DateTimeWithoutTimezone) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := utils.ParseDate(str)
	if err != nil {
		return err
	}

	*t = DateTimeWithoutTimezone{Date: parsedDate}
	return nil
}

func (t DateTimeWithoutTimezone) MarshalJSON() ([]byte, error) {
	// Устанавливаем временную зону
	location := config.TimeZone
	dateInLocation := t.Date.In(location)
	return json.Marshal(dateInLocation.Format("2006-01-02T15:04:05"))
}

type Date struct {
	Date time.Time
}

func (t *Date) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := utils.ParseDate(str)
	if err != nil {
		return err
	}

	*t = Date{Date: parsedDate}
	return nil
}

func (t Date) MarshalJSON() ([]byte, error) {
	// Устанавливаем временную зону
	location := config.TimeZone
	dateInLocation := t.Date.In(location)
	return json.Marshal(dateInLocation.Format("2006-01-02"))
}

type DateTimeOrEmpty struct {
	Date time.Time
}

func (t *DateTimeOrEmpty) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	dt := DateTime{}
	err := dt.UnmarshalJSON(data)
	if err != nil {
		return err
	}

	*t = DateTimeOrEmpty{Date: dt.Date}
	return nil
}

func (t DateTimeOrEmpty) MarshalJSON() ([]byte, error) {
	if t.Date.IsZero() {
		return json.Marshal(nil)
	}
	// Устанавливаем временную зону
	location := config.TimeZone
	dateInLocation := t.Date.In(location)
	return json.Marshal(dateInLocation.Format("2006-01-02T15:04:05-07:00"))
}
