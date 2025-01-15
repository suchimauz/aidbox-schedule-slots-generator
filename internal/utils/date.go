package utils

import "time"

// StartNextDay возвращает новую дату, где день увеличен на 1, время установлено на 00:00, а таймзона остается прежней.
func StartNextDay(t time.Time) time.Time {
	// Увеличиваем день на 1
	newDate := t.AddDate(0, 0, 1)
	// Устанавливаем время на 00:00
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	return newDate
}

// StartNextWeek возвращает новую дату, где день увеличен на 7, время установлено на 00:00, а таймзона остается прежней.
func StartNextWeek(t time.Time) time.Time {
	// Увеличиваем день на 1
	newDate := t.AddDate(0, 0, 7)
	// Устанавливаем время на 00:00
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, newDate.Location())
	return newDate
}
