package usecases

import (
	"github.com/Vaflel/lesson-counter/domain"
	"github.com/Vaflel/lesson-counter/infrastructure"
)

// ScheduleService управляет оркестрацией парсинга и валидации расписания
type ScheduleService struct {
	weekStart string
}

// ValidatingResult содержит результаты обработки расписания
type ValidatingResult struct {
	Violations []domain.Violation
	Lessons    []domain.Lesson
}

// NewScheduleService создает новый экземпляр сервиса
func NewScheduleService(weekStart string) *ScheduleService {
	return &ScheduleService{
		weekStart: weekStart,
	}
}

func (s ScheduleService) ProcessSchedule(weekStart string) (ValidatingResult, error) {
	students_repository := infrastructure.NewYAMLStudentRepository("students.yaml")
	students, _ := students_repository.LoadStudents()

	// соберём уникальные факультеты
	deptSet := make(map[string]struct{})
	for _, student := range students {
		deptSet[student.Department] = struct{}{}
	}

	departments := make([]string, 0, len(deptSet))
	for dept := range deptSet {
		departments = append(departments, dept)
	}

	// соберём уникальные группы
	set := make(map[string]struct{})
	for _, student := range students {
		set[student.Group] = struct{}{}
	}

	groups := make([]string, 0, len(set))
	for group := range set {
		groups = append(groups, group)
	}
	lessons_repository := infrastructure.NewLessonsRepository(departments, groups, weekStart)
	lessons, _ := lessons_repository.GetLessons()

	// fmt.Println(".")
	// fmt.Println(".")
	// fmt.Println(".")
	// for idx, _ := range lessons {
	// 	fmt.Printf(" %03d Дата: %s, Пара: %d, Дисциплина: %s, Студент: %s Преподаватель: %s, Кабинет: %s, Группа: %s\n",
	// 		idx+1,
	// 		lesson.Time.DateString(),
	// 		lesson.Time.Number,
	// 		lesson.Discipline,
	// 		lesson.Student,
	// 		lesson.Teacher,
	// 		lesson.Cabinet,
	// 		lesson.Group)
	// }
	// fmt.Println(".")
	// fmt.Println(".")
	// fmt.Println(".")

	valdator := domain.NewValidator(students, lessons)
	violations := valdator.ValidateSchedule()

	return ValidatingResult{
		Violations: violations,
		Lessons:    lessons,
	}, nil

}
