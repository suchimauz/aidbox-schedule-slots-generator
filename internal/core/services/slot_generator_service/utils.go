package slot_generator_service

import (
	"strings"
	"sync"
	"time"

	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"
	"github.com/suchimauz/aidbox-schedule-slots-generator/internal/utils"
)

// Функция для проверки доступности канала
func isChannelAvailable(channels []domain.ScheduleRuleChannel, check_channels string) bool {
	if check_channels == "" {
		return true
	}

	splitted_channels := strings.Split(check_channels, ",")
	for _, check_channel := range splitted_channels {
		for _, channel := range channels {
			if string(channel) == check_channel {
				return true
			}
		}
	}

	return false
}

func getSlotWeekday(scheduleRuleGlobal *domain.ScheduleRuleGlobal, date time.Time) domain.ScheduleRuleDaysOfWeek {
	date_weekday := date.Weekday()
	weekday := domain.ScheduleRuleDaysOfWeekMap[date_weekday]

	// Проверяем попадает ли дата в Замену дня недели
	// Если попадает, то заменяем текущий день недели на заменяемый день недели из производственного календаря
	for _, replacement := range scheduleRuleGlobal.Replacements {
		replacementDay := replacement.Date.Date.Format("2006-01-02")
		slotDay := date.Truncate(24 * time.Hour).Format("2006-01-02")
		if replacementDay == slotDay {
			weekday = replacement.DayOfWeek
		}
	}

	return weekday
}

// --------------------------------------------
// ПРАВИЛО 50%
// --------------------------------------------

// Функция для проверки, доступен ли слот для правила 50%
func isAvailableSlotFor50PercentRule(slot domain.Slot) bool {
	// Пропускаем слоты, которые не являются ROUTINE
	if slot.SlotType != domain.AppointmentTypeRoutine {
		return false
	}

	// Пропускаем слоты, которые не являются WEB
	if !isChannelAvailable(slot.Channel, string(domain.ScheduleRuleChannelWeb)) {
		return false
	}

	return true
}

// Обрабатывает слоты в рамках одной недели
func weekProcessing50PercentRule(currentWeekDate time.Time, nextWeekDate time.Time, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	weekSlots := make([]domain.Slot, 0)

	// Получаем все слоты, которые попадают в текущую неделю
	for _, slot := range *slots {
		// Пропускаем слоты, которые не попадают под правило 50%
		if !isAvailableSlotFor50PercentRule(slot) {
			continue
		}

		// Проверяем, попадает ли слот в текущую неделю
		if slot.StartTime.Before(currentWeekDate) || slot.StartTime.After(nextWeekDate) {
			continue
		}
		weekSlots = append(weekSlots, slot)
	}

	// Получаем все занятые слоты в текущей неделе
	bookedSlots := make([]domain.Slot, 0)
	for _, slot := range weekSlots {
		if len(slot.AppointmentIDS) > 0 {
			bookedSlots = append(bookedSlots, slot)
		}
	}

	// Вычисляем долю занятых слотов
	bookedRate := float64(len(bookedSlots)) / float64(len(weekSlots))

	// Проверяем, занята ли половина слотов в текущей неделе
	if len(weekSlots) > 0 {
		if bookedRate >= 0.5 {
			// Блокируем все свободные слоты в текущей неделе
			for _, weekSlot := range weekSlots {
				if len(weekSlot.AppointmentIDS) == 0 {
					for i := range *slots {
						slot := &(*slots)[i] // Получаем указатель на слот

						// Пропускаем слоты, которые не попадают под правило 50%
						if !isAvailableSlotFor50PercentRule(*slot) {
							continue
						}

						wg.Add(1)
						go func(slot *domain.Slot, weekSlot domain.Slot) {
							defer wg.Done()

							// Проверяем, совпадает ли слот с текущим слотом
							// Проверяем, совпадает ли слот с текущим слотом
							if slot.StartTime.Equal(weekSlot.StartTime) && slot.EndTime.Equal(weekSlot.EndTime) {
								mu.Lock()
								slot.AppointmentIDS = append(slot.AppointmentIDS, "_reserved_50_percent_rule_")
								mu.Unlock()
							}
						}(slot, weekSlot)
					}
				}
			}
		}
	}
}

// Функция для проверки правила 50%
// --------------------------------------------
// Слоты в первых двух неделях остаются как есть (первая неделя считается со текущего дня, т.е., если сегодня 10.01.2023(вторник), первой неделей будет 10.01.2023(вторник) – 16.01.2023(понедельник)).
// Начиная с 3-ей недели, берем по одной неделе.
// Для каждой взятой недели (п. 2) считаем общее количество слотов, в которых есть канал доступности web (Госуслуги).
// Для каждой взятой недели (п. 2) считаем количество занятых слотов (у которых есть соответствующая запись (Appointment)), в которых есть канал доступности web (Госуслуги).
// Высчитывает долю свободных слотов (п. 4) от общего числа слотов (п. 3).
// Если доля свободных слотов (п. 5) меньше или равна половине (0.5), т.е. занято уже больше половины всех слотов с каналом доступности web (Госуслуги), то все слоты в рамках данной недели, у которых есть канал доступности web (Госуслуги), блокируются.
// --------------------------------------------
func (s *SlotGeneratorService) apply50PercentRule(startDate time.Time, endDate time.Time, with50PercentRule bool, slots *[]domain.Slot, mu *sync.Mutex, wg *sync.WaitGroup) {
	// Если правило 50% не включено, то выходим
	if !with50PercentRule {
		return
	}

	// Правило начинает действовать с 3-ей недели
	// Добавляем к startDate 7 дней
	// И вызываем функцию StartNextWeek, которая увеличивает день на 7 и устанавливает время на 00:00
	ruleStartDate := utils.StartNextWeek(startDate.AddDate(0, 0, 7))

	// Создаем пул воркеров
	workerPool := make(chan struct{}, 20) // Ограничиваем 10 горутинами

	for currentWeekDate := ruleStartDate; currentWeekDate.Before(endDate); currentWeekDate = currentWeekDate.AddDate(0, 0, 7) {
		nextWeekDate := currentWeekDate.AddDate(0, 0, 7)
		wg.Add(1)

		// Занимаем слот в пуле
		workerPool <- struct{}{}

		go func(current, next time.Time) {
			defer func() {
				// Освобождаем слот в пуле
				<-workerPool
				wg.Done()
			}()
			weekProcessing50PercentRule(current, next, slots, mu, wg)
		}(currentWeekDate, nextWeekDate)
	}
}
