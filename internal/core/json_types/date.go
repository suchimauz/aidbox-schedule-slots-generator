package json_types

import (
	"encoding/json"
	"fmt"
	"time"
)

func parseDate(str string) (time.Time, error) {
	parsedDate, err := time.Parse(time.RFC3339, str)
	// Если не удалось пробуем дату со временем, но без таймзоны
	// По дефолту ставим UTC+3 для дат без таймзоны
	if err != nil {
		location := time.FixedZone("UTC+3", 3*60*60)
		parsedDate, err = time.ParseInLocation("2006-01-02T15:04:05", str, location)
		if err != nil {
			// Если не удалось, пробуем как дату без времени
			parsedDate, err = time.ParseInLocation("2006-01-02", str, location)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to parse time: %v", err)
			}
		}
	}

	return parsedDate, nil
}

type DateTime struct {
	Date time.Time
}

func (t *DateTime) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := parseDate(str)
	if err != nil {
		return err
	}

	*t = DateTime{Date: parsedDate}
	return nil
}

func (t DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Date.Format("2006-01-02T15:04:05+03:00"))
}

type DateTimeWithoutTimezone struct {
	Date time.Time
}

func (t *DateTimeWithoutTimezone) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := parseDate(str)
	if err != nil {
		return err
	}

	*t = DateTimeWithoutTimezone{Date: parsedDate}
	return nil
}

func (t DateTimeWithoutTimezone) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Date.Format("2006-01-02T15:04:05"))
}

type Date struct {
	Date time.Time
}

func (t *Date) UnmarshalJSON(data []byte) error {
	// Убираем кавычки вокруг строки
	str := string(data[1 : len(data)-1])

	parsedDate, err := parseDate(str)
	if err != nil {
		return err
	}

	*t = Date{Date: parsedDate}
	return nil
}

func (t Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Date.Format("2006-01-02"))
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

	return t.Date.MarshalJSON()
}
