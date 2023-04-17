package artifactcache

import (
	"fmt"
	"net/http"
)

type Cache struct {
	ID        int64  `xorm:"id pk autoincr" json:"-"`
	Key       string `xorm:"TEXT index unique(key_version)" json:"key"`
	Version   string `xorm:"TEXT unique(key_version)" json:"version"`
	Size      int64  `json:"cacheSize"`
	Complete  bool   `xorm:"index(complete_used_at)" json:"-"`
	UsedAt    int64  `xorm:"index(complete_used_at) updated" json:"-"`
	CreatedAt int64  `xorm:"index created" json:"-"`
}

// Bind implements render.Binder
func (c *Cache) Bind(_ *http.Request) error {
	if c.Key == "" {
		return fmt.Errorf("missing key")
	}
	if c.Version == "" {
		return fmt.Errorf("missing version")
	}
	return nil
}
