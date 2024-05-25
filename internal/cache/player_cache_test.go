package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPlayerCacheImpl(t *testing.T) {
	t.Run("Set and get", func(t *testing.T) {
		playerCache := NewPlayerCache(1000 * time.Second)

		longTermValueBefore := playerCache.getLongTerm("test")
		assert.False(t, longTermValueBefore.valid)

		playerCache.set("test", []byte("test"), 200)

		longTermValue := playerCache.getLongTerm("test")
		assert.Equal(t, "test", string(longTermValue.data))

		value, claimed := playerCache.getOrClaim("test")
		assert.False(t, claimed, "Expected entry to exist")
		assert.Equal(t, "test", string(value.data))
		assert.Equal(t, 200, value.statusCode)
	})

	t.Run("getOrClaim claims when missing", func(t *testing.T) {
		playerCache := NewPlayerCache(1000 * time.Second)

		value, claimed := playerCache.getOrClaim("test")
		assert.True(t, claimed, "Expected entry to not exist and get claimed")

		value, claimed = playerCache.getOrClaim("test")
		assert.False(t, claimed, "Expected entry to exist and not get claimed")
		assert.False(t, value.valid, "Expected entry to be invalid")
	})

	t.Run("delete", func(t *testing.T) {
		playerCache := NewPlayerCache(1000 * time.Second)
		playerCache.set("test", []byte("test"), 200)

		playerCache.delete("test")

		longTermValue := playerCache.getLongTerm("test")
		assert.Equal(t, "test", string(longTermValue.data), "shouldn't delete long term value")

		_, claimed := playerCache.getOrClaim("test")
		assert.True(t, claimed, "Expected to not find a value")

		longTermValueAfterClaim := playerCache.getLongTerm("test")
		assert.Equal(t, "test", string(longTermValueAfterClaim.data), "claim shouldn't change long term value")
	})

	t.Run("delete missing entry", func(t *testing.T) {
		playerCache := NewPlayerCache(1000 * time.Second)

		playerCache.delete("test")

		_, claimed := playerCache.getOrClaim("test")
		assert.True(t, claimed, "Expected to not find a value")
	})

	t.Run("wait", func(t *testing.T) {
		playerCache := NewPlayerCache(1000 * time.Second)
		playerCache.wait()
	})
}
