package server

import (
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type SyncType int

const (
	SyncTokens = iota
	SyncDocs
)

type Cache struct {
	db            *DB
	updateTimeout time.Duration
	docsMx        sync.RWMutex
	docs          map[string]Doc
	tokens        map[string]UserToken
	tokensMx      sync.RWMutex
	Ch            chan SyncType
}

func NewCache(db *DB, updateTimeout time.Duration) *Cache {
	cache := Cache{
		db:            db,
		updateTimeout: updateTimeout,
		Ch:            make(chan SyncType),
	}
	go cache.Run()
	return &cache
}

func (c *Cache) Run() {
	c.tokensSync()
	c.docsSync()

	for {
		select {
		case syncType := <-c.Ch:
			switch syncType {
			case SyncTokens:
				c.tokensSync()
			case SyncDocs:
				c.docsSync()
			}
		case <-time.After(c.updateTimeout):
			c.tokensSync()
			c.docsSync()
		}
	}
}

func (c *Cache) tokensSync() {
	c.tokensMx.Lock()
	defer c.tokensMx.Unlock()

	tokens, err := c.db.GetTokens()
	if err != nil {
		log.Fatal(err)
	}

	c.tokens = tokens
}

func (c *Cache) GetUserToken(token string) (UserToken, bool) {
	c.tokensMx.RLock()
	defer c.tokensMx.RUnlock()

	userToken, ok := c.tokens[token]
	return userToken, ok
}

func (c *Cache) GetUserTokens() map[string]UserToken {
	c.tokensMx.RLock()
	defer c.tokensMx.RUnlock()

	return c.tokens
}

func (c *Cache) GetUserTokenByUserID(userId int64) (UserToken, bool) {
	c.tokensMx.RLock()
	defer c.tokensMx.RUnlock()

	for _, ut := range c.tokens {
		if ut.UserID == userId {
			return ut, true
		}
	}
	return UserToken{}, false
}

func (c *Cache) docsSync() {
	c.docsMx.Lock()
	defer c.docsMx.Unlock()

	docs, err := c.db.GetDocs()
	if err != nil {
		log.Fatal(err)
	}
	c.docs = docs
}

func (c *Cache) getDoc(filename string) (Doc, bool) {
	c.docsMx.RLock()
	defer c.docsMx.RUnlock()

	doc, ok := c.docs[filename]
	if !ok {
		return Doc{}, false
	}
	return doc, true
}

func (c *Cache) getDocByID(id int64) (Doc, bool) {
	c.docsMx.RLock()
	defer c.docsMx.RUnlock()

	for _, d := range c.docs {
		if d.Id == id {
			return d, true
		}
	}
	return Doc{}, false
}

func (c *Cache) getDocs() map[string]Doc {
	c.docsMx.RLock()
	defer c.docsMx.RUnlock()

	return c.docs
}
