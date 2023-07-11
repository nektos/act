package artifactcache

import (
	"crypto/sha256"
	"fmt"
)

type Request struct {
	Key     string `json:"key" `
	Version string `json:"version"`
	Size    int64  `json:"cacheSize"`
}

func (c *Request) ToCache() *Cache {
	if c == nil {
		return nil
	}
	ret := &Cache{
		Key:     c.Key,
		Version: c.Version,
		Size:    c.Size,
	}
	if c.Size == 0 {
		// So the request comes from old versions of actions, like `actions/cache@v2`.
		// It doesn't send cache size. Set it to -1 to indicate that.
		ret.Size = -1
	}
	return ret
}

type Cache struct {
	ID             uint64 `json:"id" boltholdKey:"ID"`
	Key            string `json:"key" boltholdIndex:"Key"`
	Version        string `json:"version" boltholdIndex:"Version"`
	KeyVersionHash string `json:"keyVersionHash" boltholdUnique:"KeyVersionHash"`
	Size           int64  `json:"cacheSize"`
	Complete       bool   `json:"complete"`
	UsedAt         int64  `json:"usedAt" boltholdIndex:"UsedAt"`
	CreatedAt      int64  `json:"createdAt" boltholdIndex:"CreatedAt"`
}

func (c *Cache) FillKeyVersionHash() {
	c.KeyVersionHash = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", c.Key, c.Version))))
}
