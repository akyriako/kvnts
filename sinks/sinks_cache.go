package sinks

import (
	"github.com/bluele/gcache"
	"time"
)

const (
	cacheSize = 32
	cacheTTL  = 1 * time.Hour
)

type Cache struct {
	sinks gcache.Cache
}

func NewCache() *Cache {
	return &Cache{
		sinks: gcache.New(cacheSize).Expiration(cacheTTL).ARC().Build(),
	}
}

func (c *Cache) Add(namespacedName string, sink Sink, expireIn time.Duration) error {
	return c.sinks.SetWithExpire(namespacedName, sink, expireIn)
}

func (c *Cache) Contains(namespacedName string) bool {
	return c.sinks.Has(namespacedName)
}

func (c *Cache) Get(namespacedName string) (Sink, error) {
	val, err := c.sinks.Get(namespacedName)
	if err != nil {
		return nil, err
	}

	return val.(Sink), nil
}

func (c *Cache) Remove(namespacedName string) {
	c.sinks.Remove(namespacedName)
}
