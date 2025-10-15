// Package web предоставляет функции для отображения данных расписания и нарушений
package web

import (
	"bytes"
	"embed"
	"html/template"
	"strconv"
	"strings"

	"github.com/Vaflel/lesson-counter/domain"
)

//go:embed templates/index.html templates/students.html templates/edit_student.html static/*
var templates embed.FS

// Slot представляет временной слот в расписании, содержащий информацию о занятиях по разным дням недели
type Slot struct {
	Number int   // Номер временного слота (например, 1-6)
	Days   []Day // Информация о занятиях по каждому дню недели для этого слота
}

// Day представляет информацию о занятии в определенный день недели
type Day struct {
	Teacher     string // Преподаватель
	Discipline  string // Дисциплина
	Hours       string // Количество академических часов
	IsViolation bool   // Флаг, указывающий на наличие нарушения в расписании
}

// ViolationData содержит данные о нарушении расписания для конкретного студента
type ViolationData struct {
	StudentName string // Имя студента
	Group       string // Группа студента
	Year        int    // Курс студента
	Type        string // Тип нарушения
	Hours       int    // Количество академических часов нарушения
	Slots       []Slot // Временные слоты с информацией о занятиях по дням недели
}

// TemplateData содержит все данные, необходимые для отображения отчета о нарушениях
type TemplateData struct {
	WeekDateStart string          // Дата начала недели для отчета
	WeekDateEnd   string          // Дата окончания недели для отчета
	Violations    []ViolationData // Список нарушений
}

// RenderViolations генерирует HTML-представление отчета о нарушениях расписания
// Принимает список нарушений и список занятий, возвращает HTML-строку и ошибку
func RenderViolations(violations []domain.Violation, lessons []domain.Lesson) (string, error) {
	data := prepareTemplateData(violations, lessons)

	// Константа reportTemplate содержит HTML-шаблон для отображения отчета о нарушениях расписания
	// Шаблон включает таблицу с расписанием, где нарушения выделены цветом
	const reportTemplate = `
		<div style="text-align: center; margin-bottom: 20px;">
			<p>Период: с {{.WeekDateStart}} по {{.WeekDateEnd}}</p>
		</div>
		{{if .Violations}}
		{{range .Violations}}
		<h2>Студент: {{.StudentName}} (Группа: {{.Group}}, Курс: {{.Year}})</h2>
		<p><strong>Нарушение:</strong> {{.Type}} ({{.Hours}} ак.ч)</p>
		<table class="schedule-table">
			<tr>
				<th>№</th>
				<th colspan="3">Понедельник</th>
				<th colspan="3">Вторник</th>
				<th colspan="3">Среда</th>
				<th colspan="3">Четверг</th>
				<th colspan="3">Пятница</th>
				<th colspan="3">Суббота</th>
			</tr>
			<tr>
				<th></th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
				<th>Преподаватель</th><th>Дисциплина</th><th>Часы</th>
			</tr>
			{{range .Slots}}
			<tr>
				<td>{{.Number}}</td>
				{{range .Days}}
				<td {{if .IsViolation}}style="background-color: #ffcccc;"{{end}}>{{if .Teacher}}{{.Teacher}}{{else}}-{{end}}</td>
				<td {{if .IsViolation}}style="background-color: #ffcccc;"{{end}}>{{if .Discipline}}{{.Discipline}}{{else}}-{{end}}</td>
				<td {{if .IsViolation}}style="background-color: #ffcccc;"{{end}}>{{if .Hours}}{{.Hours}}{{else}}-{{end}}</td>
				{{end}}
			</tr>
			{{end}}
		</table>
		{{end}}
		{{else}}
		<p style="text-align: center; font-size: 18px;">✅<br>Отлично!<br>Нарушений в расписании не найдено.</p>
		{{end}}
	`

	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// prepareTemplateData подготавливает данные для отображения в шаблоне отчета о нарушениях
// Принимает список нарушений и список занятий, возвращает структуру TemplateData
func prepareTemplateData(violations []domain.Violation, lessons []domain.Lesson) TemplateData {
	data := TemplateData{
		WeekDateStart: "Не указано",
		WeekDateEnd:   "Не указано",
	}

	if len(lessons) > 0 {
		data.WeekDateStart = lessons[0].Time.WeekStartString()
		data.WeekDateEnd = lessons[0].Time.WeekEndString()
	}

	// dayIndex сопоставляет названия дней недели с их индексами (0-5)
	dayIndex := map[string]int{
		"понедельник": 0,
		"вторник":     1,
		"среда":       2,
		"четверг":     3,
		"пятница":     4,
		"суббота":     5,
	}

	// lessonKey представляет ключ для группировки занятий по различным параметрам
	type lessonKey struct {
		Student    string // Имя студента
		Group      string // Группа студента
		Date       string // Дата занятия
		DayName    string // Название дня недели
		Number     int    // Номер временного слота
		Discipline string // Дисциплина
		Teacher    string // Преподаватель
	}

	lessonMap := make(map[lessonKey][]domain.Lesson)
	for _, lesson := range lessons {
		key := lessonKey{
			Student:    lesson.Student,
			Group:      lesson.Group,
			Date:       lesson.Time.DateString(),
			DayName:    strings.ToLower(lesson.Time.DayName()),
			Number:     lesson.Time.Number,
			Discipline: lesson.Discipline,
			Teacher:    lesson.Teacher,
		}
		lessonMap[key] = append(lessonMap[key], lesson)
	}

	for _, v := range violations {
		violationDate := v.Date.Format("2006-01-02")
		slots := make([]Slot, 6)
		for i := range slots {
			slots[i] = Slot{
				Number: i + 1,
				Days:   make([]Day, 6),
			}
		}

		for key, lessonGroup := range lessonMap {
			if (key.Student == "" && key.Group == v.Group) || key.Student == v.StudentName {
				if slotIdx := key.Number - 1; slotIdx >= 0 && slotIdx < 6 {
					if dayIdx, exists := dayIndex[key.DayName]; exists {
						// Используем LessonTime.Hours из первого урока в группе
						hours := strconv.Itoa(lessonGroup[0].Time.Hours)
						slots[slotIdx].Days[dayIdx] = Day{
							Teacher:     key.Teacher,
							Discipline:  key.Discipline,
							Hours:       hours,
							IsViolation: key.Date == violationDate,
						}
					}
				}
			}
		}

		data.Violations = append(data.Violations, ViolationData{
			StudentName: v.StudentName,
			Group:       v.Group,
			Year:        v.Year,
			Type:        v.Type,
			Hours:       v.Hours,
			Slots:       slots,
		})
	}

	return data
}
