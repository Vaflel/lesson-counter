package usecases

import "github.com/Vaflel/lesson-counter/domain"

type LessonsRepository interface {
	GetLessons() ([]domain.Lesson, error)
}

// StudentRepository определяет интерфейс для работы с хранилищем студентов
type StudentRepository interface {
	LoadStudents() ([]domain.Student, error)
	AddStudent(student domain.Student) error
	GetStudent(name string) (domain.Student, error)
	UpdateStudent(name string, updated domain.Student) error
	DeleteStudent(name string) error
}
