package cache

import (
	"runtime"
	"sync"
)

type mockedPlayerCache struct {
	cache         map[string]cachedResponse
	longTermCache map[string]cachedResponse
}

func (playerCache *mockedPlayerCache) getOrClaim(uuid string) (cachedResponse, bool) {
	oldValue, ok := playerCache.cache[uuid]
	if ok {
		return oldValue, false
	}
	playerCache.cache[uuid] = invalid
	return invalid, true
}

func (playerCache *mockedPlayerCache) getLongTerm(uuid string) cachedResponse {
	response, ok := playerCache.longTermCache[uuid]
	if !ok {
		return invalid
	}
	return response
}

func (playerCache *mockedPlayerCache) set(uuid string, data []byte, statusCode int) {
	response := cachedResponse{data: data, statusCode: statusCode, valid: true}
	playerCache.cache[uuid] = response
	playerCache.longTermCache[uuid] = response
}

func (playerCache *mockedPlayerCache) delete(uuid string) {
	delete(playerCache.cache, uuid)
}

func (playerCache *mockedPlayerCache) wait() {
}

func NewMockedPlayerCache() *mockedPlayerCache {
	return &mockedPlayerCache{
		cache:         make(map[string]cachedResponse),
		longTermCache: make(map[string]cachedResponse),
	}
}

type mockCacheValue struct {
	cachedResponse cachedResponse
	insertedAt     int
}

type mockPlayerCacheServer struct {
	cache             map[string]mockCacheValue
	longTermCache     map[string]mockCacheValue
	cacheExpiration   int
	lock              sync.Mutex
	currentTick       int
	maxTicks          int
	numGoroutines     int
	completedThisTick int
}

type mockPlayerCacheClient struct {
	server      *mockPlayerCacheServer
	desiredTick int
}

func (cacheClient *mockPlayerCacheClient) getOrClaim(uuid string) (cachedResponse, bool) {
	oldValue, ok := cacheClient.server.cache[uuid]
	if ok && !cacheClient.server.cacheEntryHasExpired(oldValue) {
		return oldValue.cachedResponse, false
	}
	cacheClient.server.cache[uuid] = mockCacheValue{cachedResponse: invalid, insertedAt: cacheClient.server.currentTick}
	return invalid, true
}

func (cacheClient *mockPlayerCacheClient) getLongTerm(uuid string) cachedResponse {
	value, ok := cacheClient.server.longTermCache[uuid]
	if !ok {
		return invalid
	}
	return value.cachedResponse
}

func (cacheClient *mockPlayerCacheClient) set(uuid string, data []byte, statusCode int) {
	newValue := mockCacheValue{cachedResponse: cachedResponse{data: data, statusCode: statusCode, valid: true}, insertedAt: cacheClient.server.currentTick}
	cacheClient.server.cache[uuid] = newValue
	cacheClient.server.longTermCache[uuid] = newValue
}

func (cacheClient *mockPlayerCacheClient) delete(uuid string) {
	delete(cacheClient.server.cache, uuid)
}

func (cacheClient *mockPlayerCacheClient) wait() {
	if cacheClient.server.isDone() {
		panic("wait() called on a client that is already done")
	}

	cacheClient.server.lock.Lock()
	cacheClient.server.completedThisTick++
	cacheClient.server.lock.Unlock()

	cacheClient.desiredTick++

	for cacheClient.server.currentTick < cacheClient.desiredTick {
		runtime.Gosched()
	}
}

func (cacheClient *mockPlayerCacheClient) waitUntilDone() {
	for !cacheClient.server.isDone() {
		cacheClient.wait()
	}
}

func (cacheServer *mockPlayerCacheServer) cacheEntryHasExpired(entry mockCacheValue) bool {
	return entry.insertedAt+cacheServer.cacheExpiration <= cacheServer.currentTick
}

func (cacheServer *mockPlayerCacheServer) isDone() bool {
	return cacheServer.currentTick >= cacheServer.maxTicks
}

func (cacheServer *mockPlayerCacheServer) processTicks() {
	for !cacheServer.isDone() {
		if cacheServer.completedThisTick != cacheServer.numGoroutines {
			runtime.Gosched()
			continue
		}

		cacheServer.lock.Lock()
		cacheServer.completedThisTick = 0
		cacheServer.currentTick++
		cacheServer.lock.Unlock()
	}
}

func NewMockPlayerCacheServer(numGoroutines int, maxTicks int) (*mockPlayerCacheServer, []*mockPlayerCacheClient) {
	server := &mockPlayerCacheServer{
		cache:             make(map[string]mockCacheValue),
		longTermCache:     make(map[string]mockCacheValue),
		cacheExpiration:   1000,
		lock:              sync.Mutex{},
		currentTick:       0,
		maxTicks:          maxTicks,
		numGoroutines:     numGoroutines,
		completedThisTick: 0,
	}

	clients := make([]*mockPlayerCacheClient, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		clients[i] = &mockPlayerCacheClient{
			server:      server,
			desiredTick: 0,
		}
	}

	return server, clients
}
