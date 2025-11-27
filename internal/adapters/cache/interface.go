package cache

type hitResult[T any] struct {
	data    T
	valid   bool
	claimed bool
	// Channel that is closed when the entry is set or deleted
	// Only set when claimed is false and valid is false (waiting for another goroutine)
	notifyChan <-chan struct{}
}

type Cache[T any] interface {
	getOrClaim(key string) hitResult[T]
	set(key string, data T)
	delete(key string)
}
