package utils

import (
	"fmt"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/config"
)

// StartNextDay возвращает новую дату, где день увеличен на 1, время установлено на 00:00, а таймзона остается прежней.
func StartNextDay(t time.Time) time.Time {
	// Увеличиваем день на 1
	newDate := t.AddDate(0, 0, 1)
	// Устанавливаем время на 00:00
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	return newDate
}

func StartCurrentDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// StartNextWeek возвращает новую дату, где день увеличен на 7, время установлено на 00:00, а таймзона остается прежней.
func StartNextWeek(t time.Time) time.Time {
	// Увеличиваем день на 1
	newDate := t.AddDate(0, 0, 7)
	// Устанавливаем время на 00:00
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	return newDate
}

// ParseDate парсит дату из строки в формате RFC3339, если не удается, то пробует парсить дату со временем, но без таймзоны
func ParseDate(str string) (time.Time, error) {
	parsedDate, err := time.Parse(time.RFC3339, str)
	// Если не удалось пробуем дату со временем, но без таймзоны
	// По дефолту ставим таймзону из конфига
	if err != nil {
		location := config.TimeZone
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
