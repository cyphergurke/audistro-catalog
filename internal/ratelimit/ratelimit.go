package ratelimit

import (
	"net"
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastFill time.Time
	lastSeen time.Time
}

type Limiter struct {
	mu      sync.Mutex
	rps     float64
	burst   float64
	ttl     time.Duration
	buckets map[string]*bucket
}

func New(rps float64, burst int, ttl time.Duration) *Limiter {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Limiter{
		rps:     rps,
		burst:   float64(burst),
		ttl:     ttl,
		buckets: make(map[string]*bucket),
	}
}

func (l *Limiter) Allow(ip net.IP, now time.Time) bool {
	if ip == nil {
		return true
	}

	key := ip.String()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.evictExpiredLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastFill: now, lastSeen: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastFill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rps
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastFill = now
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

func (l *Limiter) evictExpiredLocked(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.ttl {
			delete(l.buckets, key)
		}
	}
}
