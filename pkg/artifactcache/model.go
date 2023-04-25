package artifactcache

import (
	"crypto/sha256"
	"fmt"
)

type Cache struct {
	ID             int64  `json:"-" boltholdKey:"ID"`
	Key            string `json:"key" boltholdIndex:"Key"`
	Version        string `json:"version" boltholdIndex:"Version"`
	KeyVersionHash string `json:"-" boltholdUnique:"KeyVersionHash"`
	Size           int64  `json:"cacheSize"`
	Complete       bool   `json:"-"`
	UsedAt         int64  `json:"-" boltholdIndex:"UsedAt"`
	CreatedAt      int64  `json:"-" boltholdIndex:"CreatedAt"`
}

func (c *Cache) FillKeyVersionHash() {
	c.KeyVersionHash = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%s", c.Key, c.Version))))
}
