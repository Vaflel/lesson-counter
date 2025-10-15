package infrastructure

import (
	"fmt"
	"os"
	"sync"

	"github.com/Vaflel/lesson-counter/domain"
	"gopkg.in/yaml.v3"
)

// StudentsConfig структура для загрузки из YAML
type StudentsConfig struct {
	Students []domain.Student `yaml:"students"`
}

// YAMLStudentRepository реализует StudentRepository для работы с YAML-файлом
type YAMLStudentRepository struct {
	filename string
	mutex    sync.RWMutex
}

// NewYAMLStudentRepository создает новый экземпляр репозитория
func NewYAMLStudentRepository(filename string) *YAMLStudentRepository {
	return &YAMLStudentRepository{
		filename: filename,
	}
}

// LoadStudents загружает список студентов из YAML файла
func (r *YAMLStudentRepository) LoadStudents() ([]domain.Student, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	data, err := os.ReadFile(r.filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var config StudentsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("не удалось распарсить YAML: %w", err)
	}

	return config.Students, nil
}

// AddStudent добавляет нового студента в YAML файл
func (r *YAMLStudentRepository) AddStudent(student domain.Student) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Загружаем текущий список студентов
	students, err := r.loadStudentsUnsafe()
	if err != nil {
		return err
	}

	// Проверяем, нет ли уже студента с таким именем
	for _, s := range students {
		if s.Name == student.Name {
			return fmt.Errorf("студент %s уже существует", student.Name)
		}
	}

	// Добавляем нового студента
	students = append(students, student)

	// Сохраняем обновленный список
	return r.saveStudentsUnsafe(students)
}

// GetStudent возвращает студента по имени
func (r *YAMLStudentRepository) GetStudent(name string) (domain.Student, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	students, err := r.loadStudentsUnsafe()
	if err != nil {
		return domain.Student{}, err
	}

	for _, s := range students {
		if s.Name == name {
			return s, nil
		}
	}

	return domain.Student{}, fmt.Errorf("студент %s не найден", name)
}

// UpdateStudent обновляет данные студента
func (r *YAMLStudentRepository) UpdateStudent(name string, updated domain.Student) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Загружаем текущий список студентов
	students, err := r.loadStudentsUnsafe()
	if err != nil {
		return err
	}

	// Ищем студента для обновления
	for i, s := range students {
		if s.Name == name {
			students[i] = updated
			return r.saveStudentsUnsafe(students)
		}
	}

	return fmt.Errorf("студент %s не найден", name)
}

// DeleteStudent удаляет студента по имени
func (r *YAMLStudentRepository) DeleteStudent(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Загружаем текущий список студентов
	students, err := r.loadStudentsUnsafe()
	if err != nil {
		return err
	}

	// Ищем и удаляем студента
	for i, s := range students {
		if s.Name == name {
			students = append(students[:i], students[i+1:]...)
			return r.saveStudentsUnsafe(students)
		}
	}

	return fmt.Errorf("студент %s не найден", name)
}

// loadStudentsUnsafe загружает студентов без блокировки (внутренний метод)
func (r *YAMLStudentRepository) loadStudentsUnsafe() ([]domain.Student, error) {
	data, err := os.ReadFile(r.filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл: %w", err)
	}

	var config StudentsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("не удалось распарсить YAML: %w", err)
	}

	return config.Students, nil
}

// saveStudentsUnsafe сохраняет студентов в YAML файл без блокировки (внутренний метод)
func (r *YAMLStudentRepository) saveStudentsUnsafe(students []domain.Student) error {
	config := StudentsConfig{Students: students}
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("не удалось сериализовать YAML: %w", err)
	}

	if err := os.WriteFile(r.filename, data, 0644); err != nil {
		return fmt.Errorf("не удалось записать файл: %w", err)
	}

	return nil
}
