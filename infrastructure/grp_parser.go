package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/Vaflel/lesson-counter/domain"
)

// LessonData представляет данные урока из API
type LessonData struct {
	WeekDayNum      int    `json:"WEEK_DAY_NUM"`
	WeekDayDate     string `json:"WEEK_DAY_DATE"`
	WeekDateStart   string `json:"WEEK_DATE_START"`
	WeekDateEnd     string `json:"WEEK_DATE_END"`
	LessonNum       int    `json:"LESSON_NUM"`
	LessonTimeStart string `json:"LESSON_TIME_START"`
	LessonTimeEnd   string `json:"LESSON_TIME_END"`
	DiscName        string `json:"DISC_NAME"`
	DiscType        string `json:"DISC_TYPE"`
	DiscSubgroup    string `json:"DISC_SUBGROUP"`
	TeacherName     string `json:"TEACHER_NAME"`
	AuditName       string `json:"AUDIT_NAME"`
	GroupName       string `json:"GROUP_NAME"`
}

// LessonListResponse представляет ответ API со списком уроков
type LessonListResponse struct {
	LessonList map[string]LessonData `json:"LessonList"`
}

const (
	scheduleURL = "https://sspi.ru/"
	aliasURL    = "https://sspi.ru/?alias=429"
)

// GroupScheduleParser обрабатывает парсинг группового расписания
type GroupScheduleParser struct {
	departmentName string
	groupName      string
	weekStart      string
	client         *http.Client
}

// NewGroupScheduleParser создаёт новый экземпляр парсера
func NewGroupScheduleParser(departmentName, groupName, weekStart string) *GroupScheduleParser {
	jar, _ := cookiejar.New(nil)
	return &GroupScheduleParser{
		departmentName: departmentName,
		groupName:      groupName,
		weekStart:      weekStart,
		client:         &http.Client{Jar: jar},
	}
}

func (gsp *GroupScheduleParser) createRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:137.0) Gecko/20100101 Firefox/137.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://sspi.ru")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://sspi.ru/?alias=429")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Priority", "u=0")

	return req, nil
}

func (gsp *GroupScheduleParser) fetchDepartments() (map[string]string, error) {
	req, err := gsp.createRequest("GET", aliasURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := gsp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	departments := make(map[string]string)
	doc.Find("label[for='ChangeFakultet']").Each(func(i int, s *goquery.Selection) {
		s.Next().Find("option").Each(func(j int, opt *goquery.Selection) {
			text := strings.TrimSpace(opt.Text())
			value, _ := opt.Attr("value")
			if text != "" && value != "" {
				departments[text] = value
			}
		})
	})

	return departments, nil
}

func (gsp *GroupScheduleParser) fetchGroups(departmentID string) (map[string]string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("FakultetId", departmentID); err != nil {
		return nil, err
	}

	contentType := writer.FormDataContentType()
	writer.Close()

	req, err := gsp.createRequest("POST", scheduleURL+"plugins/AutoRasp/SearchGroup.php", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := gsp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	groups := make(map[string]string)
	doc.Find("option").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		value, _ := s.Attr("value")
		if text != "" && value != "" {
			groups[text] = value
		}
	})

	return groups, nil
}

func (gsp *GroupScheduleParser) fetchLessons(groupID string) ([]LessonData, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("GroupId", groupID); err != nil {
		return nil, err
	}
	if gsp.weekStart != "" {
		if err := writer.WriteField("WeekNum", gsp.weekStart); err != nil {
			return nil, err
		}
	}

	contentType := writer.FormDataContentType()
	writer.Close()

	req, err := gsp.createRequest("POST", scheduleURL+"plugins/AutoRasp/GroupLessonList.php", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := gsp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	bodyStr := string(bodyBytes)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
		return nil, fmt.Errorf("API returned HTML instead of JSON. Possibly expired session or invalid GroupId")
	}

	var lessonResponse LessonListResponse
	if err := json.Unmarshal(bodyBytes, &lessonResponse); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %w (first 200 chars: %s)", err, truncate(bodyStr, 200))
	}

	lessons := make([]LessonData, 0, len(lessonResponse.LessonList))
	for _, lesson := range lessonResponse.LessonList {
		lessons = append(lessons, lesson)
	}

	return lessons, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func deduplicateLessons(lessonsData []LessonData) []domain.Lesson {
	// Группируем уроки по дате, номеру пары и подгруппе
	type lessonKey struct {
		date      string
		lessonNum int
	}
	groups := make(map[lessonKey][]LessonData)

	for _, data := range lessonsData {
		if data.DiscName == "Индивидуальные занятия" {
			continue
		}
		key := lessonKey{date: data.WeekDayDate, lessonNum: data.LessonNum}
		groups[key] = append(groups[key], data)
	}

	var result []domain.Lesson
	for _, group := range groups {
		if len(group) == 1 && group[0].DiscSubgroup == "0" {
			// Одиночный урок без подгруппы
			data := group[0]
			lesson := createLesson(data)
			result = append(result, lesson)
		} else {
			// Объединяем уроки с подгруппами
			mergedLessons := mergeLessons(group)
			lesson := createLesson(mergedLessons)
			result = append(result, lesson)
		}
	}

	return result
}

func createLesson(data LessonData) domain.Lesson {
	date, _ := time.Parse("2006-01-02", data.WeekDayDate)
	startTime, _ := time.Parse("15:04", data.LessonTimeStart)
	endTime, _ := time.Parse("15:04", data.LessonTimeEnd)

	discipline := data.DiscName
	if data.DiscType != "" {
		discipline = fmt.Sprintf("%s [%s]", data.DiscName, data.DiscType)
	}

	for pairHalf := 1; pairHalf <= 2; pairHalf++ {

	}
	return domain.Lesson{
		Time:       domain.NewLessonTime(date, data.LessonNum, 0, 2, startTime, endTime),
		Discipline: discipline,
		Teacher:    data.TeacherName,
		Cabinet:    data.AuditName,
		Group:      data.GroupName,
		Student:    "",
	}

}

func mergeLessons(data []LessonData) LessonData {
	if len(data) == 0 {
		return LessonData{}
	}

	first := data[0]

	// парсинг даты/времени — оставляю, чтобы сохранить побочный эффект проверки формата
	_, _ = time.Parse("2006-01-02", first.WeekDayDate)
	_, _ = time.Parse("15:04", first.LessonTimeStart)
	_, _ = time.Parse("15:04", first.LessonTimeEnd)

	disciplines := make([]string, len(data))
	teachers := make([]string, len(data))
	cabinets := make([]string, len(data))

	for i, d := range data {
		discipline := d.DiscName
		if d.DiscType != "" {
			discipline = fmt.Sprintf("%s [%s]", d.DiscName, d.DiscType)
		}
		disciplines[i] = discipline
		teachers[i] = d.TeacherName
		cabinets[i] = d.AuditName
	}

	merged := first // копируем первый элемент как базу

	merged.DiscName = strings.Join(disciplines, " / ")
	merged.TeacherName = strings.Join(teachers, " & ")
	merged.AuditName = strings.Join(cabinets, " / ")

	// если нужно пометить, что это объединённый урок, можно изменить другие поля,
	// например DiscSubgroup, но оставляю их без изменений по умолчанию.

	return merged
}

// Parse извлекает расписание группы
func (gsp *GroupScheduleParser) Parse() ([]domain.Lesson, error) {
	departments, err := gsp.fetchDepartments()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch departments: %w", err)
	}

	departmentID, ok := departments[gsp.departmentName]
	if !ok {
		return nil, fmt.Errorf("department '%s' not found", gsp.departmentName)
	}

	groups, err := gsp.fetchGroups(departmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}

	groupID, ok := groups[gsp.groupName]
	if !ok {
		return nil, fmt.Errorf("group '%s' not found", gsp.groupName)
	}

	lessonsData, err := gsp.fetchLessons(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lessons: %w", err)
	}

	return deduplicateLessons(lessonsData), nil
}
