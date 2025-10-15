package domain

import "time"

// Student содержит информацию о студенте
type Student struct {
	Name       string
	Group      string
	Department string
	Year       int
}

// Violation описывает нарушение в расписании студента
type Violation struct {
	StudentName string
	Group       string
	Year        int
	Date        time.Time // дата
	Type        string    // "Превышение нагрузки" или "Превышение окон"
	Hours       int       // количество академических часов
}

// NewViolation создает новое нарушение
func NewViolation(studentName, group string, year int, date time.Time, violationType string, hours int) Violation {
	return Violation{
		StudentName: studentName,
		Group:       group,
		Year:        year,
		Date:        date,
		Type:        violationType,
		Hours:       hours,
	}
}

type Lesson struct {
	Time       LessonTime
	Discipline string
	Teacher    string
	Cabinet    string
	Group      string
	Student    string
}

// LessonTime содержит информацию о дате и времени пары
type LessonTime struct {
	Date      time.Time // дата пары
	Number    int       // номер пары
	PairHalf  int       // половина пары (0 - обе половинки 1/2 - первая или вторая)
	Hours     int       // количество часов в уроке
	StartTime time.Time // начало пары
	EndTime   time.Time // конец пары
}

// DateString возвращает дату в формате "2006-01-02"
func (t LessonTime) DateString() string {
	return t.Date.Format("2006-01-02")
}

// DayName возвращает название дня недели
func (t LessonTime) DayName() string {
	names := []string{
		"воскресенье", "понедельник", "вторник",
		"среда", "четверг", "пятница", "суббота",
	}
	return names[t.Date.Weekday()]
}

// StartTimeString возвращает время начала в формате "15:04"
func (t LessonTime) StartTimeString() string {
	return t.StartTime.Format("15:04")
}

// EndTimeString возвращает время конца в формате "15:04"
func (t LessonTime) EndTimeString() string {
	return t.EndTime.Format("15:04")
}

func (t LessonTime) WeekStart() time.Time {
	offset := (int(t.Date.Weekday()) + 6) % 7
	start := time.Date(t.Date.Year(), t.Date.Month(), t.Date.Day(), 0, 0, 0, 0, t.Date.Location())
	return start.AddDate(0, 0, -offset)
}

// WeekEnd возвращает дату конца недели (воскресенье)
func (t LessonTime) WeekEnd() time.Time {
	return t.WeekStart().AddDate(0, 0, 6)
}

// WeekStartString возвращает дату начала недели в формате "2006-01-02"
func (t LessonTime) WeekStartString() string {
	return t.WeekStart().Format("2006-01-02")
}

// WeekEndString возвращает дату конца недели в формате "2006-01-02"
func (t LessonTime) WeekEndString() string {
	return t.WeekEnd().Format("2006-01-02")
}

// NewLessonTime создает LessonTime с датой и временем начала/конца
func NewLessonTime(date time.Time, number, pairHalf int, hours int, startTime, endTime time.Time) LessonTime {
	return LessonTime{
		Date:      date,
		Number:    number,
		PairHalf:  pairHalf,
		Hours:     hours,
		StartTime: startTime,
		EndTime:   endTime,
	}
}
