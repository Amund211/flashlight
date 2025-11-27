package cache

import "sync"

type basicCacheEntry[T any] struct {
	data  T
	valid bool
}

type basicCache[T any] struct {
	cache       map[string]basicCacheEntry[T]
	notifyChans map[string]chan struct{}
	lock        sync.Mutex
}

func (c *basicCache[T]) getOrClaim(key string) hitResult[T] {
	c.lock.Lock()
	defer c.lock.Unlock()

	oldValue, ok := c.cache[key]
	if ok {
		var notifyChan <-chan struct{}
		// Only create a wait channel if the value is not valid (waiting for another goroutine to populate it)
		if !oldValue.valid {
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
	c.lock.Lock()
	c.cache[key] = basicCacheEntry[T]{data: data, valid: true}
	// Close and delete notification channel while holding lock
	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
	c.lock.Unlock()
}

func (c *basicCache[T]) delete(key string) {
	c.lock.Lock()
	delete(c.cache, key)
	// Close and delete notification channel while holding lock
	if ch, ok := c.notifyChans[key]; ok {
		close(ch)
		delete(c.notifyChans, key)
	}
	c.lock.Unlock()
}

func NewBasicCache[T any]() *basicCache[T] {
	return &basicCache[T]{
		cache:       make(map[string]basicCacheEntry[T]),
		notifyChans: make(map[string]chan struct{}),
	}
}
