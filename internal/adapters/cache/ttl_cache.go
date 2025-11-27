package cache

import (
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type tllCacheEntry[T any] struct {
	data  T
	valid bool
}

type ttlCache[T any] struct {
	cache       *ttlcache.Cache[string, tllCacheEntry[T]]
	notifyChans map[string]chan struct{}
	notifyLock  sync.Mutex
}

func (c *ttlCache[T]) getNotifyChan(key string) <-chan struct{} {
	c.notifyLock.Lock()
	defer c.notifyLock.Unlock()

	if ch, ok := c.notifyChans[key]; ok {
		return ch
	}

	ch := make(chan struct{})
	c.notifyChans[key] = ch
	return ch
}

func (c *ttlCache[T]) closeNotifyChan(key string) {
	c.notifyLock.Lock()
	defer c.notifyLock.Unlock()

	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
}

func (c *ttlCache[T]) getOrClaim(key string) hitResult[T] {
	invalid := tllCacheEntry[T]{valid: false}
	item, existed := c.cache.GetOrSet(key, invalid)

	var notifyChan <-chan struct{}
	// Only create a wait channel if the value is not valid (waiting for another goroutine to populate it)
	if existed && !item.Value().valid {
		notifyChan = c.getNotifyChan(key)
	}

	return hitResult[T]{
		data:       item.Value().data,
		valid:      item.Value().valid,
		claimed:    !existed,
		notifyChan: notifyChan,
	}
}

func (c *ttlCache[T]) set(key string, data T) {
	c.cache.Set(key, tllCacheEntry[T]{data: data, valid: true}, ttlcache.DefaultTTL)
	c.closeNotifyChan(key)
}

func (c *ttlCache[T]) delete(key string) {
	c.cache.Delete(key)
	c.closeNotifyChan(key)
}

func NewTTLCache[T any](ttl time.Duration) Cache[T] {
	cache := ttlcache.New[string, tllCacheEntry[T]](
		ttlcache.WithTTL[string, tllCacheEntry[T]](ttl),
		ttlcache.WithDisableTouchOnHit[string, tllCacheEntry[T]](),
	)
	go cache.Start()
	return &ttlCache[T]{
		cache:       cache,
		notifyChans: make(map[string]chan struct{}),
	}
}
