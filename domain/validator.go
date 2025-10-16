package domain

import (
	"fmt"
)

// Validator содержит данные и методы для проверки расписания
type Validator struct {
	students []Student
	lessons  []Lesson
}

// NewValidator создаёт новый Validator
func NewValidator(students []Student, lessons []Lesson) *Validator {
	return &Validator{
		students: students,
		lessons:  lessons,
	}
}
func (v *Validator) ValidateSchedule() []Violation {
	violations := []Violation{}

	for _, student := range v.students {
		studentSchedule := v.collectStudentSchedule(student)
		scheduleByDate := v.groupByDate(studentSchedule)

		for _, dayLessons := range scheduleByDate {
			studentViolations := v.validateDay(student, dayLessons)
			violations = append(violations, studentViolations...)
		}
	}

	fmt.Printf("Найдено нарушений: %d\n", len(violations))
	return violations
}

// collectStudentSchedule собирает расписание студента (индивидуальные + групповые пары)
func (v *Validator) collectStudentSchedule(student Student) []Lesson {
	schedule := []Lesson{}

	for _, lesson := range v.lessons {
		if lesson.Student != "" && lesson.Student == student.Name {
			schedule = append(schedule, lesson)
			continue
		}
		if lesson.Student == "" && lesson.Group == student.Group {
			schedule = append(schedule, lesson)
		}
	}

	return schedule
}

// groupByDate группирует уроки по дате
func (v *Validator) groupByDate(lessons []Lesson) map[string][]Lesson {
	result := make(map[string][]Lesson)

	for _, lesson := range lessons {
		date := lesson.Time.DateString()
		result[date] = append(result[date], lesson)
	}

	return result
}

// validateDay проверяет один день расписания студента
func (v *Validator) validateDay(student Student, dayLessons []Lesson) []Violation {
	violations := []Violation{}

	if len(dayLessons) == 0 {
		return violations
	}

	date := dayLessons[0].Time.Date

	// Нагрузка (сумма часов)
	totalHours := v.calculateLoad(dayLessons)
	if totalHours > 10 {
		violation := NewViolation(
			student.Name,
			student.Group,
			student.Year,
			date,
			"Превышение нагрузки",
			totalHours,
		)
		violations = append(violations, violation)
	}

	// Окна — считаем пустые PairHalf
	gapPairs := v.calculateGaps(dayLessons)
	maxGaps := 4
	if student.Year == 1 {
		maxGaps = 2
	}
	if gapPairs > maxGaps {
		violation := NewViolation(
			student.Name,
			student.Group,
			student.Year,
			date,
			"Превышение окон",
			gapPairs,
		)
		violations = append(violations, violation)
	}

	return violations
}

// calculateLoad считает суммарную нагрузку по сумме Hours у уроков
func (v *Validator) calculateLoad(dayLessons []Lesson) int {
	total := 0
	for _, lesson := range dayLessons {
		total += lesson.Time.Hours
	}
	return total
}

// calculateGaps считает количество пустых PairHalf между минимальной и максимальной занятой ячейкой
func (v *Validator) calculateGaps(dayLessons []Lesson) int {
	if len(dayLessons) == 0 {
		return 0
	}

	minSlot := int(^uint(0) >> 1) // max int - очень большая циферь, можно было ставить что-то в районе 15
	maxSlot := -1
	occupiedSlots := make(map[int]bool)

	for _, lesson := range dayLessons {
		if lesson.Time.PairHalf == 0 {
			// Пара занимает обе половинки
			slot1 := (lesson.Time.Number - 1) * 2
			slot2 := slot1 + 1
			occupiedSlots[slot1] = true
			occupiedSlots[slot2] = true

			if slot1 < minSlot {
				minSlot = slot1
			}
			if slot2 > maxSlot {
				maxSlot = slot2
			}
		} else {
			// Пара занимает только одну половинку (1 или 2)
			slot := (lesson.Time.Number-1)*2 + (lesson.Time.PairHalf - 1)
			occupiedSlots[slot] = true

			if slot < minSlot {
				minSlot = slot
			}
			if slot > maxSlot {
				maxSlot = slot
			}
		}
	}

	totalSlots := maxSlot - minSlot + 1
	occupied := len(occupiedSlots)
	return totalSlots - occupied
}
