package slot_generator_service

import "github.com/suchimauz/aidbox-schedule-slots-generator/internal/core/domain"

type SlotSlice []domain.Slot

// quickSort — функция для сортировки SlotSlice
func (s SlotSlice) quickSort() SlotSlice {
	if len(s) < 2 {
		return s
	}

	// Выбираем опорный элемент
	pivot := s[len(s)/2]

	// Разделяем слайс на три части
	less := SlotSlice{}
	equal := SlotSlice{}
	greater := SlotSlice{}

	for _, slot := range s {
		if slot.StartTime.Before(pivot.StartTime) {
			less = append(less, slot)
		} else if slot.StartTime.Equal(pivot.StartTime) {
			equal = append(equal, slot)
		} else {
			greater = append(greater, slot)
		}
	}

	// Рекурсивно сортируем подмассивы и объединяем их
	return append(append(less.quickSort(), equal...), greater.quickSort()...)
}
