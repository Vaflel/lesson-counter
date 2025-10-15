package infrastructure

import (
	"log"
	"sync"
	"time"

	"github.com/Vaflel/lesson-counter/domain"
)

// GroupLessonsCache представляет объект кэша для хранения групповых уроков в оперативной памяти.
// Кэш хранит уроки по ключу weekStart с временем истечения (TTL 30 минут).
// Доступ к кэшу синхронизирован с помощью мьютекса для безопасной работы в многопоточной среде.
type GroupLessonsCache struct {
	mu   sync.Mutex
	data map[string]struct {
		lessons []domain.Lesson
		expiry  time.Time // Время истечения кэша для записи
	}
}

// NewGroupLessonsCache создаёт новый экземпляр кэша групповых уроков.
func NewGroupLessonsCache() *GroupLessonsCache {
	return &GroupLessonsCache{
		data: make(map[string]struct {
			lessons []domain.Lesson
			expiry  time.Time
		}),
	}
}

// Get возвращает кэшированные уроки для указанного weekStart, если они существуют и не истекли.
// Если кэш истёк, запись удаляется. Возвращает уроки и флаг успеха (true, если кэш валиден).
func (c *GroupLessonsCache) Get(weekStart string) ([]domain.Lesson, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.data[weekStart]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.expiry) {
		delete(c.data, weekStart)
		return nil, false
	}

	return entry.lessons, true
}

// Set сохраняет уроки в кэш для указанного weekStart с TTL 30 минут.
func (c *GroupLessonsCache) Set(weekStart string, lessons []domain.Lesson) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[weekStart] = struct {
		lessons []domain.Lesson
		expiry  time.Time
	}{
		lessons: lessons,
		expiry:  time.Now().Add(30 * time.Minute),
	}
}

// LessonsRepositoryImpl реализует интерфейс LessonsRepository для получения уроков.
// Хранит списки отделений, групп, начало недели и список уроков.
// Использует встроенный кэш для групповых уроков в оперативной памяти.
type LessonsRepositoryImpl struct {
	departments []string
	groups      []string
	weekStart   string
	lessons     []domain.Lesson
	mu          sync.Mutex
	cache       *GroupLessonsCache // Встроенный объект кэша для групповых уроков
}

// NewLessonsRepository создаёт новый репозиторий уроков с инициализацией кэша.
func NewLessonsRepository(departments []string, groups []string, weekStart string) *LessonsRepositoryImpl {
	return &LessonsRepositoryImpl{
		departments: departments,
		groups:      groups,
		weekStart:   weekStart,
		lessons:     make([]domain.Lesson, 0),
		cache:       NewGroupLessonsCache(),
	}
}

// GetLessons возвращает список индивидуальных и групповых уроков.
// Сначала парсит индивидуальные уроки, затем проверяет кэш на наличие групповых.
// Если кэш валиден, использует его; иначе парсит групповые уроки, сохраняет в кэш и возвращает.
// Парсинг групповых уроков выполняется параллельно в горутинах.
func (r *LessonsRepositoryImpl) GetLessons() ([]domain.Lesson, error) {
	// Парсинг индивидуальных уроков (без кэширования)
	individualParser := NewIndividualScheduleParser()
	if individualLessons, err := individualParser.Parse(); err == nil {
		r.mu.Lock()
		r.lessons = append(r.lessons, individualLessons...)
		r.mu.Unlock()
	} else {
		log.Printf("Error parsing individual schedule: %v", err)
	}

	// Проверка кэша для групповых уроков
	if cachedLessons, ok := r.cache.Get(r.weekStart); ok {
		r.mu.Lock()
		r.lessons = append(r.lessons, cachedLessons...)
		r.mu.Unlock()
		return r.lessons, nil
	}

	// Парсинг групповых уроков в горутинах, если кэш не валиден
	var wg sync.WaitGroup
	var groupLessons []domain.Lesson
	var groupMu sync.Mutex

	for _, department := range r.departments {
		for _, group := range r.groups {
			wg.Add(1)
			go func(dep, grp string) {
				defer wg.Done()
				gsp := NewGroupScheduleParser(dep, grp, r.weekStart)
				if lessons, err := gsp.Parse(); err == nil {
					groupMu.Lock()
					groupLessons = append(groupLessons, lessons...)
					groupMu.Unlock()
					r.mu.Lock()
					r.lessons = append(r.lessons, lessons...)
					r.mu.Unlock()
				} else {
					log.Printf("Error parsing group schedule for group %s: %v", grp, err)
				}
			}(department, group)
		}
	}

	wg.Wait()

	// Сохранение групповых уроков в кэш
	r.cache.Set(r.weekStart, groupLessons)

	return r.lessons, nil
}
