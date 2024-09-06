package cache

import (
	"encoding/json"
	"github.com/patrickmn/go-cache"
	"log"
	"time"
)

type allCache struct {
	secrets *cache.Cache
}

const (
	defaultExpiration = 9999999 * time.Minute
	purgeTime         = 9999999 * time.Minute
)

func NewCache() *allCache {
	Cache := cache.New(cache.NoExpiration, 0)
	return &allCache{
		secrets: Cache,
	}
}

func (c *allCache) Read(id string) (item []byte, ok bool) {
	secret, ok := c.secrets.Get(id)
	if ok {
		log.Println("from cache")
		res, err := json.Marshal(secret.(Secret))
		if err != nil {
			log.Fatal("Error")
		}
		return res, true
	}
	return nil, false
}

func (c *allCache) update(id string, product Product) {
	c.products.Set(id, product, cache.DefaultExpiration)
}
