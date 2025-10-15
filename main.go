package main

import (
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/Vaflel/lesson-counter/infrastructure"
	"github.com/Vaflel/lesson-counter/web"
)

// openBrowser opens the specified URL in the default browser on Windows
func openBrowser(url string) {
	err := exec.Command("cmd", "/c", "start", url).Start()
	if err != nil {
		log.Printf("Не удалось открыть браузер: %v", err)
		log.Printf("Откройте вручную: %s", url)
	}
}

func main() {
	studentRepo := infrastructure.NewYAMLStudentRepository("students.yaml")

	server := web.NewServer(studentRepo)

	port := 8060

	go func() {

		time.Sleep(500 * time.Millisecond)
		url := fmt.Sprintf("http://localhost:%d", port)
		log.Printf("Открываем браузер: %s\n", url)
		openBrowser(url)
	}()

	if err := server.Start(port); err != nil {
		log.Fatalf("Ошибка запуска веб-сервера: %v", err)
	}
}
