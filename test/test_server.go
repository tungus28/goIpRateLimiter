package main

import (
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	serverURL := "https://devrating.org/"

	// Отправляем запрос каждые 100 миллисекунд (10 запросов в секунду)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	log.Println("🚀 Тестирование Rate Limiter запущено...")

	requestCount := 0

	for range ticker.C {
		requestCount++

		// Ограничим тест 20 запросами, чтобы он не шел вечно
		if requestCount > 20 {
			log.Println("🏁 Тест завершен.")
			break
		}

		// Оборачиваем тело цикла в анонимную функцию.
		// Это нужно для того, чтобы defer resp.Body.Close() срабатывал
		// сразу на каждом шаге цикла, а не копился до самого конца main.
		func(reqNum int) {
			start := time.Now()
			resp, err := http.Get(serverURL)
			if err != nil {
				log.Printf("[%d] Ошибка подключения: %v", reqNum, err)
				return // Безопасный выход из анонимной функции (цикл пойдет дальше)
			}
			// Закрываем соединение сразу после завершения этой итерации
			defer resp.Body.Close()

			diff := time.Since(start)
			log.Printf("[%d] Статус ответа: %s, потрачено: %v", reqNum, resp.Status, diff)

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			log.Println("Body:\n", string(body))

		}(requestCount)
	}
}
