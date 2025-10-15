package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Vaflel/lesson-counter/domain"
	"github.com/Vaflel/lesson-counter/usecases"
)

type Server struct {
	studentRepo  usecases.StudentRepository
	mu           sync.Mutex
	isProcessing bool
	reportReady  bool
	violations   []domain.Violation
	lessons      []domain.Lesson
	server       *http.Server
}

type CheckResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type CheckRequest struct {
	WeekStart string `json:"weekStart"`
}

type StatusResponse struct {
	IsProcessing bool   `json:"isProcessing"`
	ReportReady  bool   `json:"reportReady"`
	Report       string `json:"report,omitempty"`
}

func NewServer(studentRepo usecases.StudentRepository) *Server {
	return &Server{
		studentRepo: studentRepo,
	}
}

func (s *Server) Start(port int) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/check", s.handleCheck)
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/students", s.handleStudents)
	http.HandleFunc("/students/edit/", s.handleEditStudent)
	http.HandleFunc("/students/delete/", s.handleDeleteStudent)
	http.HandleFunc("/shutdown", s.handleShutdown)
	http.HandleFunc("/static/", s.handleStatic)

	addr := fmt.Sprintf(":%d", port)
	s.server = &http.Server{Addr: addr}
	log.Printf("🚀 Сервер запущен на http://localhost%s", addr)
	return s.server.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFS(templates, "templates/index.html")
	if err != nil {
		log.Printf("Ошибка загрузки шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	data := struct {
		IsProcessing bool
		Report       template.HTML // Изменяем тип на template.HTML
	}{
		IsProcessing: s.isProcessing,
	}
	if s.reportReady {
		report, err := RenderViolations(s.violations, s.lessons)
		if err != nil {
			log.Printf("Ошибка рендеринга отчета: %v", err)
			s.mu.Unlock()
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		data.Report = template.HTML(report) // Преобразуем строку в template.HTML
	}
	s.mu.Unlock()

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Ошибка рендеринга шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
	}
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	var reqData CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if reqData.WeekStart == "" {
		http.Error(w, "Не указана дата начала недели", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.isProcessing {
		s.mu.Unlock()
		http.Error(w, "Обработка уже выполняется", http.StatusTooManyRequests)
		return
	}
	s.isProcessing = true
	s.reportReady = false
	s.mu.Unlock()

	go func() {
		service := usecases.NewScheduleService(reqData.WeekStart)
		result, err := service.ProcessSchedule(reqData.WeekStart)
		s.mu.Lock()
		defer s.mu.Unlock()

		s.isProcessing = false
		if err != nil {
			log.Printf("Ошибка обработки расписания: %v", err)
			return
		}

		s.violations = result.Violations
		s.lessons = result.Lessons
		s.reportReady = true
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CheckResponse{
		Success: true,
		Message: "Обработка запущена",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	response := StatusResponse{
		IsProcessing: s.isProcessing,
		ReportReady:  s.reportReady,
	}
	if s.reportReady {
		var err error
		response.Report, err = RenderViolations(s.violations, s.lessons)
		if err != nil {
			log.Printf("Ошибка рендеринга отчета: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleStudents(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates, "templates/students.html")
	if err != nil {
		log.Printf("Ошибка загрузки шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Неверные данные формы", http.StatusBadRequest)
			return
		}

		name := r.FormValue("name")
		group := r.FormValue("group")
		department := r.FormValue("department")
		year, err := strconv.Atoi(r.FormValue("year"))
		if err != nil || name == "" {
			http.Error(w, "Поле name и корректный year обязательны", http.StatusBadRequest)
			return
		}

		if err := s.studentRepo.AddStudent(domain.Student{
			Name:       name,
			Group:      group,
			Department: department,
			Year:       year,
		}); err != nil {
			log.Printf("Ошибка добавления студента: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/students", http.StatusSeeOther)
		return
	}

	students, err := s.studentRepo.LoadStudents()
	if err != nil {
		log.Printf("Ошибка загрузки студентов: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	sort.Slice(students, func(i, j int) bool {
		if students[i].Year != students[j].Year {
			return students[i].Year < students[j].Year
		}
		return strings.ToLower(students[i].Name) < strings.ToLower(students[j].Name)
	})

	if err := tmpl.Execute(w, struct{ Students []domain.Student }{students}); err != nil {
		log.Printf("Ошибка рендеринга шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
	}
}

func (s *Server) handleEditStudent(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Path)
	tmpl, err := template.ParseFS(templates, "templates/edit_student.html")
	if err != nil {
		log.Printf("Ошибка загрузки шаблона: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		student, err := s.studentRepo.GetStudent(name)
		if err != nil {
			log.Printf("Ошибка получения студента: %v", err)
			http.Error(w, "Студент не найден", http.StatusNotFound)
			return
		}
		if err := tmpl.Execute(w, student); err != nil {
			log.Printf("Ошибка рендеринга шаблона: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Ошибка обработки формы", http.StatusBadRequest)
			return
		}

		newName := r.FormValue("name")
		group := r.FormValue("group")
		department := r.FormValue("department")
		year, err := strconv.Atoi(r.FormValue("year"))
		if err != nil || newName == "" || group == "" || department == "" || year <= 0 {
			http.Error(w, "Все поля обязательны, курс должен быть > 0", http.StatusBadRequest)
			return
		}

		if err := s.studentRepo.UpdateStudent(name, domain.Student{
			Name:       newName,
			Group:      group,
			Department: department,
			Year:       year,
		}); err != nil {
			log.Printf("Ошибка обновления студента: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/students", http.StatusSeeOther)
		return
	}

	http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
}

func (s *Server) handleDeleteStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	if err := s.studentRepo.DeleteStudent(filepath.Base(r.URL.Path)); err != nil {
		log.Printf("Ошибка удаления студента: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{true, "Сервер завершает работу"})

	go func() {
		log.Println("Завершение работы сервера...")
		if err := s.server.Shutdown(r.Context()); err != nil {
			log.Printf("Ошибка при завершении работы: %v", err)
		}
		os.Exit(0)
	}()
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := "static/" + strings.TrimPrefix(r.URL.Path, "/static/")
	content, err := templates.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	if strings.HasSuffix(r.URL.Path, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(r.URL.Path, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	}
	w.Write(content)
}
