package ratelimit

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"tokentracer-proxy/pkg/auth"
	"tokentracer-proxy/pkg/db"
)

var (
	defaultMinuteLimit int
	defaultDailyLimit  int
)

func init() {
	defaultMinuteLimit = getEnvInt("RATE_LIMIT_MINUTE", 0)
	defaultDailyLimit = getEnvInt("RATE_LIMIT_DAILY", 0)
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

type userLimits struct {
	minute    int
	daily     int
	fetchedAt time.Time
}

var (
	limitsCache   = make(map[int]userLimits)
	limitsCacheMu sync.RWMutex
	limitsCacheTTL = 1 * time.Minute
)

func getUserLimits(userID int) (minuteLimit, dailyLimit int) {
	limitsCacheMu.RLock()
	cached, ok := limitsCache[userID]
	limitsCacheMu.RUnlock()

	if ok && time.Since(cached.fetchedAt) < limitsCacheTTL {
		return resolveLimit(cached.minute, defaultMinuteLimit), resolveLimit(cached.daily, defaultDailyLimit)
	}

	// Fetch from DB
	// TODO: cache
	var dbMinute, dbDaily int
	err := db.Pool.QueryRow(context.Background(),
		"SELECT rate_limit_minute, rate_limit_daily FROM users WHERE id = $1", userID).
		Scan(&dbMinute, &dbDaily)
	if err != nil {
		// On error, use server defaults
		return defaultMinuteLimit, defaultDailyLimit
	}

	limitsCacheMu.Lock()
	limitsCache[userID] = userLimits{minute: dbMinute, daily: dbDaily, fetchedAt: time.Now()}
	limitsCacheMu.Unlock()

	return resolveLimit(dbMinute, defaultMinuteLimit), resolveLimit(dbDaily, defaultDailyLimit)
}

// resolveLimit returns the effective limit. If the user value is 0, fall back to
// the server default. A final value of 0 means unlimited.
func resolveLimit(userValue, serverDefault int) int {
	if userValue != 0 {
		return userValue
	}
	return serverDefault
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(auth.KeyUser).(int)
		if !ok {
			log.Printf("rate limit middleware: missing user context")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		minuteLimit, dailyLimit := getUserLimits(userID)

		// 1. Check Daily Limit (0 = unlimited)
		if dailyLimit > 0 {
			dailyCount, err := getDailyCount(userID)
			if err != nil {
				log.Printf("rate limit middleware: daily count error for user %d: %v", userID, err)
				http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
				return
			}
			if dailyCount >= dailyLimit {
				http.Error(w, "Daily rate limit exceeded.", http.StatusTooManyRequests)
				return
			}
		}

		// 2. Per-Minute Limit (0 = unlimited)
		if minuteLimit > 0 {
			if isMinuteLimitExceeded(userID, minuteLimit) {
				http.Error(w, "Per-minute rate limit exceeded.", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func getDailyCount(userID int) (int, error) {
	var count int
	err := db.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM request_logs WHERE user_id = $1 AND created_at >= CURRENT_DATE",
		userID).Scan(&count)
	return count, err
}

var (
	minuteBuckets = make(map[string]int)
	bucketMu      sync.Mutex
)

func isMinuteLimitExceeded(userID int, limit int) bool {
	minute := time.Now().Format("2006-01-02 15:04")
	key := fmt.Sprintf("%d:%s", userID, minute)

	bucketMu.Lock()
	defer bucketMu.Unlock()

	count := minuteBuckets[key]
	if count >= limit {
		return true
	}

	minuteBuckets[key] = count + 1

	// Prune expired entries (keys from old minutes)
	if len(minuteBuckets) > 10000 {
		currentMinute := time.Now().Format("2006-01-02 15:04")
		for k := range minuteBuckets {
			// Make sure it doesn't have the current minute in the key
			if !strings.HasSuffix(k, ":"+currentMinute) {
				delete(minuteBuckets, k)
			}
		}
	}

	return false
}
