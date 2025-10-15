package infrastructure

// Пакет infrastructure содержит реализацию парсера индивидуальных расписаний
// студентов, обрабатывающего XLS-файлы с расписаниями индивидуальных занятий.
// Основная задача пакета — извлечение данных об уроках (Lessons) из файлов и
// преобразование их в структуры, определённые в пакете domain. Парсер работает
// с файлами в формате XLS, содержащими расписания преподавателей, и извлекает
// информацию о датах, времени, дисциплинах, кабинетах, группах и студентах.
//
// Основной компонент пакета — структура IndividualScheduleParser, которая
// отвечает за парсинг файлов и формирование списка уроков (domain.Lesson).
// Парсер поддерживает обработку нескольких преподавателей в одном файле,
// объединение уроков с одинаковыми параметрами и обработку сложных форматов
// данных, таких как разделённые имена студентов или нестандартные названия
// дисциплин.
//
// Использование:
// 1. Создайте экземпляр парсера с помощью NewIndividualScheduleParser().
// 2. Вызовите метод Parse() для обработки всех XLS-файлов в текущей директории.
// 3. Получите список уроков (domain.Lesson) или ошибку в случае неудачи.
//
// Ограничения:
// - Парсер работает только с XLS-файлами, закодированными в windows-1251.
// - Предполагается, что структура таблицы соответствует определённому формату
//   (например, наличие заголовка "Расписание индивидуальных занятий", строк с
//   датами, номерами пар и т.д.).
// - Время пар фиксировано и задаётся в pairTime внутри структуры парсера.
//
// Зависимости:
// - Пакет "github.com/extrame/xls" для работы с XLS-файлами.
// - Пакет "github.com/Vaflel/lesson-counter/domain" для структур данных.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Vaflel/lesson-counter/domain"
	"github.com/extrame/xls"
)

// IndividualScheduleParser обрабатывает парсинг индивидуальных расписаний из XLS-файлов.
// Содержит пути к файлам и карту времени пар для преобразования номеров уроков во временные интервалы.
type IndividualScheduleParser struct {
	filePaths []string
	pairTime  map[int][2]string
}

// NewIndividualScheduleParser создаёт новый экземпляр парсера с предустановленной
// картой времени пар (pairTime), где каждому номеру пары соответствует время начала и конца.
func NewIndividualScheduleParser() *IndividualScheduleParser {
	return &IndividualScheduleParser{
		pairTime: map[int][2]string{
			1: {"08:30", "10:00"},
			2: {"10:10", "11:40"},
			3: {"11:50", "13:20"},
			4: {"13:40", "15:10"},
			5: {"15:20", "16:50"},
			6: {"16:55", "18:25"},
			7: {"18:30", "20:00"},
		},
	}
}

// Parse обрабатывает все XLS-файлы в текущей директории и возвращает список уроков (domain.Lesson).
// Метод загружает пути к файлам, парсит их содержимое и объединяет уроки с одинаковыми параметрами.
// Возвращает ошибку, если файлы не найдены или данные некорректны.
func (p *IndividualScheduleParser) Parse() ([]domain.Lesson, error) {
	if err := p.loadFilePaths(); err != nil {
		return nil, fmt.Errorf("ошибка загрузки файлов: %w", err)
	}

	var allLessons []domain.Lesson

	for _, filePath := range p.filePaths {
		file, err := xls.Open(filePath, "windows-1251")
		if err != nil {
			continue
		}

		sheet := file.GetSheet(0)
		if sheet == nil {
			continue
		}

		teacherRows := p.findTeacherRows(sheet)

		for _, teacherRow := range teacherRows {
			teacherName, err := p.extractTeacher(sheet, teacherRow)
			if err != nil {
				continue
			}

			dayColumns, err := p.extractDayColumns(sheet, teacherRow)
			if err != nil {
				continue
			}

			lessonsRowStart := teacherRow + 6

			for _, dayColumn := range dayColumns {
				date, err := p.extractDate(sheet, lessonsRowStart, dayColumn)
				if err != nil {
					continue
				}

				for lessonRow := lessonsRowStart; lessonRow < int(sheet.MaxRow); lessonRow++ {
					lessonNumber, err := p.extractLessonNumber(sheet, lessonRow)
					if err != nil || lessonNumber == 0 {
						break
					}

					disciplineName, err := p.extractDiscipline(sheet, lessonRow, dayColumn)
					if err != nil {
						continue
					}

					cabinetNumber, err := p.extractCabinet(sheet, lessonRow, dayColumn)
					if err != nil {
						continue
					}

					groupName, err := p.extractGroup(sheet, lessonRow, dayColumn)
					if err != nil {
						continue
					}

					studentNames, err := p.extractStudentNames(sheet, lessonRow, dayColumn)
					if err != nil {
						continue
					}

					startTime, endTime := p.parsePairTime(date, lessonNumber)

					for index, studentName := range studentNames {
						lessonGroup := groupName
						if studentName == "" {
							lessonGroup = ""
						}
						hoursStub := 1
						pairHalf := index + 1
						lesson := domain.Lesson{
							Time:       domain.NewLessonTime(date, lessonNumber, pairHalf, hoursStub, startTime, endTime),
							Discipline: disciplineName,
							Teacher:    teacherName,
							Cabinet:    cabinetNumber,
							Group:      lessonGroup,
							Student:    studentName,
						}
						allLessons = append(allLessons, lesson)
					}
				}
			}
		}
	}

	if len(allLessons) == 0 {
		return nil, fmt.Errorf("не найдено уроков в файлах")
	}

	mergedLessons := p.mergeTeachers(allLessons)
	joinedLessons := p.joinIndLessons(mergedLessons)
	return joinedLessons, nil

}

// findTeacherRows находит строки, содержащие "Преподаватель" в колонке 0.
// Возвращает список индексов строк, где начинаются блоки с именами преподавателей.
func (p *IndividualScheduleParser) findTeacherRows(sheet *xls.WorkSheet) []int {
	var rows []int
	for rowIndex := 0; rowIndex < int(sheet.MaxRow); rowIndex++ {
		cell := sheet.Row(rowIndex).Col(0)
		if strings.HasPrefix(strings.TrimSpace(cell), "Преподаватель") {
			rows = append(rows, rowIndex)
		}
	}
	return rows
}

// extractDate извлекает дату занятия из таблицы, используя строку заголовка дней и год из периода.
// Формирует полную дату в формате "02.01.2006" и возвращает её как time.Time.
func (p *IndividualScheduleParser) extractDate(sheet *xls.WorkSheet, lessonsRowStart, dayCol int) (time.Time, error) {
	daysHeaderRow := lessonsRowStart - 3

	// Получаем год из периода
	periodRow := sheet.Row(daysHeaderRow - 2).Col(7)
	if periodRow == "" {
		return time.Time{}, fmt.Errorf("не удалось получить строку периода (строка %d, колонка 7)", daysHeaderRow-2)
	}

	reYear := regexp.MustCompile(`\b(\d{4})\b`)
	year := reYear.FindString(periodRow)
	if year == "" {
		return time.Time{}, fmt.Errorf("не удалось найти год в строке '%s'", periodRow)
	}

	// Получаем дату
	dateRow := sheet.Row(daysHeaderRow + 1).Col(dayCol)
	if dateRow == "" {
		return time.Time{}, fmt.Errorf("не удалось получить строку даты (строка %d, колонка %d)", daysHeaderRow+1, dayCol)
	}

	dateStr := strings.TrimSpace(strings.TrimPrefix(dateRow, "Дата:"))
	// Удаляем возможную точку в конце перед добавлением года
	dateStr = strings.TrimSuffix(dateStr, ".")
	fullDate := fmt.Sprintf("%s.%s", dateStr, year)

	date, err := time.Parse("02.01.2006", fullDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("ошибка парсинга даты '%s': %w", fullDate, err)
	}

	return date, nil
}

// extractLessonNumber извлекает номер урока из указанной строки таблицы.
// Возвращает 0 и ошибку, если ячейка пуста или содержит некорректные данные.
func (p *IndividualScheduleParser) extractLessonNumber(sheet *xls.WorkSheet, row int) (int, error) {
	cell := sheet.Row(row).Col(1)
	if cell == "" {
		return 0, fmt.Errorf("пустая ячейка номера урока")
	}
	num, err := strconv.Atoi(cell)
	if err != nil {
		return 0, fmt.Errorf("неверный формат номера урока: %w", err)
	}
	return num, nil
}

// parsePairTime преобразует номер урока во время начала и конца на основе предустановленной карты pairTime.
// Объединяет дату с временем для получения полного time.Time.
func (p *IndividualScheduleParser) parsePairTime(date time.Time, lessonNumber int) (time.Time, time.Time) {
	times := p.pairTime[lessonNumber]

	// Парсим время и объединяем с датой
	startHM, _ := time.Parse("15:04", times[0])
	endHM, _ := time.Parse("15:04", times[1])

	// Создаём полное время с датой
	startTime := time.Date(date.Year(), date.Month(), date.Day(),
		startHM.Hour(), startHM.Minute(), 0, 0, date.Location())
	endTime := time.Date(date.Year(), date.Month(), date.Day(),
		endHM.Hour(), endHM.Minute(), 0, 0, date.Location())

	return startTime, endTime
}

// extractDiscipline извлекает название дисциплины из ячейки, очищая данные от информации о кабинете и группе.
// Нормализует названия дисциплин, сопоставляя их с заранее определёнными значениями.
// Игнорирует дисциплины "Народный танец (практикум)" и "Народный танец".
func (p *IndividualScheduleParser) extractDiscipline(sheet *xls.WorkSheet, row, column int) (string, error) {
	cell := sheet.Row(row).Col(column + 1)
	if cell == "" {
		return "", fmt.Errorf("пустая ячейка дисциплины")
	}

	disciplineData := strings.Join(strings.Fields(cell), " ")

	// Удаляем информацию о кабинете
	reCabinet := regexp.MustCompile(`\s*\(.*?\)`)
	clean := reCabinet.ReplaceAllString(disciplineData, "")

	// Удаляем информацию о группе, если она есть
	reGroup := regexp.MustCompile(`\s*(МД-\d{2}-о)\s*`)
	clean = reGroup.ReplaceAllString(clean, "")

	// Нормализуем строку
	clean = strings.TrimSpace(clean)

	// Игнорируем дисциплины "Народный танец (практикум)" и "Народный танец"
	if clean == "Народный танец (практикум)" || clean == "Народный танец" {
		return "", fmt.Errorf("дисциплина '%s' игнорируется", clean)
	}

	// Сопоставляем очищенное название дисциплины
	switch clean {
	case "Дир.хор.подг.", "Хор.дир.":
		return "Дирижирование", nil
	case "Муз.инстр.испол.", "Муз.инстр.подг.", "Муз.инстр.подгот.", "Муз.инстр.исполн.", "Муз.инстр.подготов.":
		return "Муз. Инструмент", nil
	case "Вокал.", "Вокал. исполн.", "Вокал. подг.":
		return "Вокал", nil
	case "Аккомпанемент":
		return "Аккомпанемент", nil
	default:
		return clean, nil
	}
}

// extractTeacher извлекает имя преподавателя из таблицы, используя регулярное выражение.
// Возвращает форматированное имя в формате "Фамилия И.О." или "Unknown", если данные некорректны.
func (p *IndividualScheduleParser) extractTeacher(sheet *xls.WorkSheet, teacherRow int) (string, error) {
	cell := sheet.Row(teacherRow).Col(0)
	if cell == "" {
		return "", fmt.Errorf("пустая ячейка имени преподавателя")
	}

	teacherName := strings.TrimSpace(cell)
	reTeacher := regexp.MustCompile(`Преподаватель\s*(\p{L}+)\s*(\p{L}\.\p{L}\.)(?:\s*Подпись.*)?`)
	matches := reTeacher.FindStringSubmatch(teacherName)
	if len(matches) < 3 {
		return "Unknown", nil
	}

	tchr := fmt.Sprintf("%s %s", matches[1], matches[2])
	// fmt.Println(teacherRow, tchr)
	return tchr, nil
}

// extractCabinet извлекает номер кабинета из ячейки с данными дисциплины.
// Форматирует номер в виде "К3-номер" или возвращает ошибку, если номер не найден.
func (p *IndividualScheduleParser) extractCabinet(sheet *xls.WorkSheet, row, column int) (string, error) {
	cell := sheet.Row(row).Col(column + 1)
	if cell == "" {
		return "", fmt.Errorf("пустая ячейка кабинета")
	}

	disciplineData := strings.Join(strings.Fields(cell), " ")
	reCabinet := regexp.MustCompile(`\(.*?(\d{2,3}).*?\)`)
	matches := reCabinet.FindStringSubmatch(disciplineData)
	if len(matches) < 2 {
		return "", fmt.Errorf("номер кабинета не найден")
	}

	return fmt.Sprintf("К3-%s", matches[1]), nil
}

// extractGroup извлекает название группы из ячейки с данными дисциплины.
// Возвращает пустую строку, если группа не найдена, без ошибки.
func (p *IndividualScheduleParser) extractGroup(sheet *xls.WorkSheet, row, column int) (string, error) {
	cell := sheet.Row(row).Col(column + 1)
	if cell == "" {
		return "", fmt.Errorf("пустая ячейка группы")
	}

	disciplineData := strings.Join(strings.Fields(cell), " ")
	reGroup := regexp.MustCompile(`(МД-\d{2}-о)`)
	matches := reGroup.FindStringSubmatch(disciplineData)
	if len(matches) < 2 {
		return "", nil // Возвращаем пустую строку вместо ошибки
	}

	return matches[1], nil
}

// extractStudentNames извлекает имена студентов из ячейки, разделяя их по символу "/".
// Возвращает список имён или два одинаковых имени, если студент один.
func (p *IndividualScheduleParser) extractStudentNames(sheet *xls.WorkSheet, row, column int) ([]string, error) {
	cell := sheet.Row(row).Col(column)
	if cell == "" {
		return nil, fmt.Errorf("пустая ячейка имени студента")
	}

	studentNameCell := strings.Join(strings.Fields(cell), " ")

	if strings.Contains(studentNameCell, "/") {
		names := strings.Split(studentNameCell, "/")
		var result []string
		for _, name := range names {
			trimmed := strings.TrimSpace(name)
			result = append(result, trimmed)
		}
		return result, nil
	}

	return []string{studentNameCell, studentNameCell}, nil
}

// extractDayColumns определяет столбцы, соответствующие дням недели, в таблице преподавателя.
// Возвращает список индексов столбцов или ошибку, если столбцы не найдены.
func (p *IndividualScheduleParser) extractDayColumns(sheet *xls.WorkSheet, teacherRow int) ([]int, error) {
	var columns []int
	daysHeaderRow := teacherRow + 3

	for day := 2; day < 30; day += 3 {
		cell := sheet.Row(daysHeaderRow).Col(day)
		if cell == "" {
			break
		}
		columns = append(columns, day)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("не найдено столбцов с днями")
	}

	return columns, nil
}

// mergeTeachers объединяет уроки с одинаковыми параметрами, но разными преподавателями.
// Формирует уникальный ключ для каждого урока и объединяет имена преподавателей через "& " в алфавитном порядке.
func (p *IndividualScheduleParser) mergeTeachers(lessons []domain.Lesson) []domain.Lesson {
	groups := make(map[string][]domain.Lesson)
	for _, lesson := range lessons {
		key := fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s",
			lesson.Time.DateString(),
			lesson.Time.Number,
			lesson.Time.PairHalf,
			lesson.Discipline,
			lesson.Cabinet,
			lesson.Group,
			lesson.Student,
		)
		groups[key] = append(groups[key], lesson)
	}
	var result []domain.Lesson
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		merged := group[0]
		if len(group) > 1 {
			teacherSet := make(map[string]bool)
			for _, lesson := range group {
				if lesson.Teacher != "" {
					teacherSet[lesson.Teacher] = true
				}
			}
			teachers := make([]string, 0, len(teacherSet))
			for teacher := range teacherSet {
				teachers = append(teachers, teacher)
			}
			// Сортируем преподавателей в алфавитном порядке
			sort.Strings(teachers)
			merged.Teacher = strings.Join(teachers, "& ")
		}
		result = append(result, merged)
	}
	return result
}

// loadFilePaths загружает пути ко всем XLS-файлам в текущей директории.
// Сохраняет пути в поле filePaths структуры парсера. Возвращает ошибку, если файлы не найдены.
func (p *IndividualScheduleParser) loadFilePaths() error {
	dataDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("не удалось получить текущую директорию: %w", err)
	}

	err = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xls") {
			p.filePaths = append(p.filePaths, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("не удалось загрузить пути к файлам: %w", err)
	}

	if len(p.filePaths) == 0 {
		return fmt.Errorf("не найдено .xls файлов в директории %s", dataDir)
	}

	return nil
}

// joinIndLessons объединяет индивидуальные занятия студентов в полные пары
func (p *IndividualScheduleParser) joinIndLessons(lessons []domain.Lesson) []domain.Lesson {
	// Группируем уроки по студенту, дате и номеру пары
	type lessonKey struct {
		student string
		date    string
		number  int
	}

	lessonsMap := make(map[lessonKey]map[int]domain.Lesson) // key -> pairHalf -> lesson

	// Собираем все индивидуальные уроки в map
	for _, lesson := range lessons {
		key := lessonKey{
			student: lesson.Student,
			date:    lesson.Time.DateString(),
			number:  lesson.Time.Number,
		}

		if lessonsMap[key] == nil {
			lessonsMap[key] = make(map[int]domain.Lesson)
		}
		lessonsMap[key][lesson.Time.PairHalf] = lesson
	}

	// Формируем результирующий список
	result := make([]domain.Lesson, 0, len(lessonsMap))

	for _, pairMap := range lessonsMap {
		lesson1, hasFirst := pairMap[1]
		lesson2, hasSecond := pairMap[2]

		var resultLesson domain.Lesson

		if hasFirst && hasSecond {
			// Обе половинки есть - объединяем
			resultLesson = lesson1
			resultLesson.Time.PairHalf = 0
			resultLesson.Time.Hours = 2

			// Проверяем, одинаковые ли дисциплины
			if lesson1.Discipline == lesson2.Discipline {
				resultLesson.Discipline = lesson1.Discipline
			} else {
				resultLesson.Discipline = lesson1.Discipline + "/" + lesson2.Discipline
			}

			// Проверяем, одинаковые ли преподаватели
			if lesson1.Teacher == lesson2.Teacher {
				resultLesson.Teacher = lesson1.Teacher
			} else {
				resultLesson.Teacher = lesson1.Teacher + "/" + lesson2.Teacher
			}

			// Проверяем, одинаковые ли кабинеты
			if lesson1.Cabinet == lesson2.Cabinet {
				resultLesson.Cabinet = lesson1.Cabinet
			} else {
				resultLesson.Cabinet = lesson1.Cabinet + "/" + lesson2.Cabinet
			}

			// Время начала берем от первой половинки, время конца - от второй
			resultLesson.Time.EndTime = lesson2.Time.EndTime
		} else if hasFirst {
			// Только первая половинка
			resultLesson = lesson1
			resultLesson.Discipline = lesson1.Discipline + "/"
			resultLesson.Teacher = lesson1.Teacher + "/"
			resultLesson.Cabinet = lesson1.Cabinet + "/"
		} else if hasSecond {
			// Только вторая половинка
			resultLesson = lesson2
			resultLesson.Discipline = "/" + lesson2.Discipline
			resultLesson.Teacher = "/" + lesson2.Teacher
			resultLesson.Cabinet = "/" + lesson2.Cabinet
		}

		result = append(result, resultLesson)
	}

	return result
}
