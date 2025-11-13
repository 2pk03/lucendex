package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRateLimitExceeded = fmt.Errorf("rate limit exceeded")
)

// Plan limits (requests per minute)
const (
	FreePlanLimit       = 100
	ProPlanLimit        = 1000
	EnterprisePlanLimit = 10000
)

type RateLimiter struct {
	kv KVStore
	db DB
}

type KVStore interface {
	IncrementRateLimit(partnerID string, ttl time.Duration) (int64, error)
}

func NewRateLimiter(kv KVStore, db DB) *RateLimiter {
	return &RateLimiter{
		kv: kv,
		db: db,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract partner from context (set by auth middleware)
		partnerID, ok := ctx.Value(ContextKeyPartnerID).(uuid.UUID)
		if !ok {
			writeError(w, http.StatusInternalServerError, "missing partner context")
			return
		}

		partner, ok := ctx.Value(ContextKeyPartner).(*Partner)
		if !ok {
			writeError(w, http.StatusInternalServerError, "missing partner context")
			return
		}

		// Get limit based on plan
		limit := rl.getLimitForPlan(partner.Plan)

		// Check rate limit
		count, err := rl.kv.IncrementRateLimit(partnerID.String(), 60*time.Second)
		if err != nil {
			// Log error but allow request (fail open)
			// In production, might want to fail closed
			next.ServeHTTP(w, r)
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprint(limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprint(max(0, limit-int(count))))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprint(time.Now().Add(60*time.Second).Unix()))

		if count > int64(limit) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, ErrRateLimitExceeded.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getLimitForPlan(plan string) int {
	switch plan {
	case "free":
		return FreePlanLimit
	case "pro":
		return ProPlanLimit
	case "enterprise":
		return EnterprisePlanLimit
	default:
		return FreePlanLimit
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
