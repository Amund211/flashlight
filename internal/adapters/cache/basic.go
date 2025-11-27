package cache

import "sync"

type basicCacheEntry[T any] struct {
	data  T
	valid bool
}

type basicCache[T any] struct {
	cache        map[string]basicCacheEntry[T]
	cacheLock    sync.Mutex
	notifyChans  map[string]chan struct{}
	notifyLock   sync.Mutex
}

func (c *basicCache[T]) getNotifyChan(key string) <-chan struct{} {
	c.notifyLock.Lock()
	defer c.notifyLock.Unlock()

	if ch, ok := c.notifyChans[key]; ok {
		return ch
	}

	ch := make(chan struct{})
	c.notifyChans[key] = ch
	return ch
}

func (c *basicCache[T]) closeNotifyChan(key string) {
	c.notifyLock.Lock()
	defer c.notifyLock.Unlock()

	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
}

func (c *basicCache[T]) getOrClaim(key string) hitResult[T] {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	oldValue, ok := c.cache[key]
	if ok {
		var notifyChan <-chan struct{}
		// Only create a wait channel if the value is not valid (waiting for another goroutine to populate it)
		if !oldValue.valid {
			notifyChan = c.getNotifyChan(key)
		}
		return hitResult[T]{
			data:       oldValue.data,
			valid:      oldValue.valid,
			claimed:    false,
			notifyChan: notifyChan,
		}
	}

	c.cache[key] = basicCacheEntry[T]{valid: false}
	return hitResult[T]{
		valid:      false,
		claimed:    true,
		notifyChan: nil,
	}
}

func (c *basicCache[T]) set(key string, data T) {
	c.cacheLock.Lock()
	c.cache[key] = basicCacheEntry[T]{data: data, valid: true}
	c.cacheLock.Unlock()

	c.closeNotifyChan(key)
}

func (c *basicCache[T]) delete(key string) {
	c.cacheLock.Lock()
	delete(c.cache, key)
	c.cacheLock.Unlock()

	c.closeNotifyChan(key)
}

func NewBasicCache[T any]() *basicCache[T] {
	return &basicCache[T]{
		cache:       make(map[string]basicCacheEntry[T]),
		notifyChans: make(map[string]chan struct{}),
	}
}
