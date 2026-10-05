package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// client хранит лимитер и время его последнего использования
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipIpRateLimiter управляет лимитерами для всех IP-адресов
type ipRateLimiter struct {
	ips sync.Map
}

// создание коллекции лимитеров на основе синк мапы
func newIPRateLimiter() *ipRateLimiter {
	i := &ipRateLimiter{}
	// Запускаем фоновую очистку старых IP, чтобы память не переполнялась
	go i.cleanupOldIPs()
	return i
}

// получение/создание лимитера для кокретнго ip из синк мапы
func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	//если ip уже есть в синк мапе, обновляем время и возвращаем
	val, exists := i.ips.Load(ip)
	if exists {
		c := val.(*client)
		c.lastSeen = time.Now()
		return c.limiter
	}

	//если нет еще такого ip - создаем новый лимитер и настраиваем через rate.Every
	// Настройки лимита: 5 запросов в секунду (добавляем токен каждые 0.2 секунды), максимальный всплеск (burst) — 10 запросов
	limiter := rate.NewLimiter(rate.Every(200*time.Millisecond), 10)

	i.ips.Store(ip, &client{
		limiter:  limiter,
		lastSeen: time.Now(),
	})

	return limiter
}

// Очистка IP, которые не делали запросов больше 3 минут
func (i *ipRateLimiter) cleanupOldIPs() {
	for {
		//запускаем очитску раз в минуту
		time.Sleep(1 * time.Minute)
		//пробегаемся по синк мапе через встроенную фунцию Range
		i.ips.Range(func(key, value any) bool {
			c := value.(*client)
			if time.Since(c.lastSeen) > 3*time.Minute {
				i.ips.Delete(key)
			}
			return true
		})
	}
}

// Middleware для ограничения запросов по IP
func limitByIP(limiterManager *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем IP-адрес клиента
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Берем лимитер конкретно для этого IP
		limiter := limiterManager.getLimiter(ip)

		// Проверяем, превысил ли этот IP лимит
		if !limiter.Allow() {
			http.Error(w, "Слишком много запросов. Подождите.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()
	limiterManager := newIPRateLimiter()

	// Обычный эндпоинт, например, главная страница или API
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Привет! Это обычный HTTP-ответ.")
	})

	// Оборачиваем все маршруты в наш IP-лимитер
	wrappedMux := limitByIP(limiterManager, mux)

	fmt.Println("Сервер запущен на :8080...")
	if err := http.ListenAndServe(":8080", wrappedMux); err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	}
}
