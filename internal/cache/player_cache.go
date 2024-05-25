package cache

import (
	"github.com/jellydator/ttlcache/v3"
	"time"
)

type cachedResponse struct {
	data       []byte
	statusCode int
	valid      bool
}

var invalid = cachedResponse{valid: false}

type PlayerCache interface {
	getOrClaim(uuid string) (cachedResponse, bool)
	getLongTerm(uuid string) cachedResponse
	set(uuid string, data []byte, statusCode int)
	delete(uuid string)
	wait()
}

type playerCacheImpl struct {
	cache         *ttlcache.Cache[string, cachedResponse]
	longTermCache *ttlcache.Cache[string, cachedResponse]
}

func (playerCache *playerCacheImpl) getOrClaim(uuid string) (cachedResponse, bool) {
	item, existed := playerCache.cache.GetOrSet(uuid, invalid)
	return item.Value(), !existed
}

func (playerCache *playerCacheImpl) getLongTerm(uuid string) cachedResponse {
	item := playerCache.longTermCache.Get(uuid)
	if item == nil {
		return invalid
	}
	response := item.Value()
	return response
}

func (playerCache *playerCacheImpl) set(uuid string, data []byte, statusCode int) {
	response := cachedResponse{data: data, statusCode: statusCode, valid: true}
	playerCache.cache.Set(uuid, response, ttlcache.DefaultTTL)
	playerCache.longTermCache.Set(uuid, response, ttlcache.DefaultTTL)
}

func (playerCache *playerCacheImpl) delete(uuid string) {
	playerCache.cache.Delete(uuid)
}

func (playerCache *playerCacheImpl) wait() {
	time.Sleep(50 * time.Millisecond)
}

func NewPlayerCache(ttl time.Duration) PlayerCache {
	playerTTLCache := ttlcache.New[string, cachedResponse](
		ttlcache.WithTTL[string, cachedResponse](ttl),
		ttlcache.WithDisableTouchOnHit[string, cachedResponse](),
	)
	go playerTTLCache.Start()

	longTermCache := ttlcache.New[string, cachedResponse](
		ttlcache.WithTTL[string, cachedResponse](30*24*time.Hour),
		ttlcache.WithDisableTouchOnHit[string, cachedResponse](),
	)
	go longTermCache.Start()
	return &playerCacheImpl{cache: playerTTLCache, longTermCache: longTermCache}
}
