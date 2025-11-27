package cache

import (
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type ttlCacheEntry[T any] struct {
	data  T
	valid bool
}

type ttlCache[T any] struct {
	cache       *ttlcache.Cache[string, ttlCacheEntry[T]]
	notifyChans map[string]chan struct{}
	lock        sync.Mutex
}

func (c *ttlCache[T]) getOrClaim(key string) hitResult[T] {
	invalid := ttlCacheEntry[T]{valid: false}
	item, existed := c.cache.GetOrSet(key, invalid)

	c.lock.Lock()
	defer c.lock.Unlock()

	var notifyChan <-chan struct{}
	// Only create a wait channel if the value is not valid (waiting for another goroutine to populate it)
	if existed && !item.Value().valid {
		// Create notification channel if it doesn't exist
		if ch, exists := c.notifyChans[key]; exists {
			notifyChan = ch
		} else {
			ch := make(chan struct{})
			c.notifyChans[key] = ch
			notifyChan = ch
		}
	}

	return hitResult[T]{
		data:       item.Value().data,
		valid:      item.Value().valid,
		claimed:    !existed,
		notifyChan: notifyChan,
	}
}

func (c *ttlCache[T]) set(key string, data T) {
	c.cache.Set(key, ttlCacheEntry[T]{data: data, valid: true}, ttlcache.DefaultTTL)

	c.lock.Lock()
	// Close and delete notification channel while holding lock
	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
	c.lock.Unlock()
}

func (c *ttlCache[T]) delete(key string) {
	c.cache.Delete(key)

	c.lock.Lock()
	// Close and delete notification channel while holding lock
	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
	c.lock.Unlock()
}

func NewTTLCache[T any](ttl time.Duration) Cache[T] {
	cache := ttlcache.New[string, ttlCacheEntry[T]](
		ttlcache.WithTTL[string, ttlCacheEntry[T]](ttl),
		ttlcache.WithDisableTouchOnHit[string, ttlCacheEntry[T]](),
	)
	go cache.Start()
	return &ttlCache[T]{
		cache:       cache,
		notifyChans: make(map[string]chan struct{}),
	}
}
