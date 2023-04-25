package artifactcache

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"go.etcd.io/bbolt"

	"github.com/nektos/act/pkg/common"
)

const (
	urlBase = "/_apis/artifactcache"
)

type Handler struct {
	db       *bolthold.Store
	storage  *Storage
	router   *httprouter.Router
	listener net.Listener
	logger   logrus.FieldLogger

	gc   atomic.Bool
	gcAt time.Time

	outboundIP string
}

func StartHandler(dir, outboundIP string, port uint16, logger logrus.FieldLogger) (*Handler, error) {
	if logger == nil {
		discard := logrus.New()
		discard.Out = io.Discard
		logger = discard
	}
	logger = logger.WithField("module", "artifactcache")

	h := &Handler{}

	if dir == "" {
		if home, err := os.UserHomeDir(); err != nil {
			return nil, err
		} else {
			dir = filepath.Join(home, ".cache", "actcache")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	if db, err := bolthold.Open(filepath.Join(dir, "bolt.db"), 0o755, &bolthold.Options{
		// TODO: debug coder
		Encoder: xml.Marshal,
		Decoder: xml.Unmarshal,
		Options: &bbolt.Options{
			Timeout:      5 * time.Second,
			NoGrowSync:   bbolt.DefaultOptions.NoGrowSync,
			FreelistType: bbolt.DefaultOptions.FreelistType,
		},
	}); err != nil {
		return nil, err
	} else {
		h.db = db
	}

	storage, err := NewStorage(filepath.Join(dir, "cache"))
	if err != nil {
		return nil, err
	}
	h.storage = storage

	if outboundIP != "" {
		h.outboundIP = outboundIP
	} else if ip := common.GetOutboundIP(); ip == nil {
		return nil, fmt.Errorf("unable to determine outbound IP address")
	} else {
		h.outboundIP = ip.String()
	}

	router := httprouter.New()
	router.GET(urlBase+"/cache", h.middleware(h.find))
	router.POST(urlBase+"/caches", h.middleware(h.reserve))
	router.PATCH(urlBase+"/caches/:id", h.middleware(h.upload))
	router.POST(urlBase+"/caches/:id", h.middleware(h.commit))
	router.GET(urlBase+"/artifacts/:id", h.middleware(h.get))
	router.POST(urlBase+"/clean", h.middleware(h.clean))

	h.router = router

	h.gcCache()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port)) // listen on all interfaces
	if err != nil {
		return nil, err
	}
	go func() {
		if err := http.Serve(listener, h.router); err != nil {
			logger.Errorf("http serve: %v", err)
		}
	}()
	h.listener = listener

	return h, nil
}

func (h *Handler) ExternalURL() string {
	// TODO: make the external url configurable if necessary
	return fmt.Sprintf("http://%s:%d",
		h.outboundIP,
		h.listener.Addr().(*net.TCPAddr).Port)
}

// GET /_apis/artifactcache/cache
func (h *Handler) find(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	keys := strings.Split(r.URL.Query().Get("keys"), ",")
	version := r.URL.Query().Get("version")

	cache, err := h.findCache(r.Context(), keys, version)
	if err != nil {
		h.responseJson(w, r, 500, err)
		return
	}
	if cache == nil {
		h.responseJson(w, r, 204)
		return
	}

	if ok, err := h.storage.Exist(cache.ID); err != nil {
		h.responseJson(w, r, 500, err)
		return
	} else if !ok {
		_ = h.db.Delete(cache.ID, cache)
		h.responseJson(w, r, 204)
		return
	}
	h.responseJson(w, r, 200, map[string]any{
		"result":          "hit",
		"archiveLocation": fmt.Sprintf("%s%s/artifacts/%d", h.ExternalURL(), urlBase, cache.ID),
		"cacheKey":        cache.Key,
	})
}

// POST /_apis/artifactcache/caches
func (h *Handler) reserve(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	cache := &Cache{}
	if err := json.NewDecoder(r.Body).Decode(cache); err != nil {
		h.responseJson(w, r, 400, err)
		return
	}

	cache.FillKeyVersionHash()
	if err := h.db.FindOne(cache, bolthold.Where("KeyVersionHash").Eq(cache.KeyVersionHash)); err != nil {
		if errors.Is(err, bolthold.ErrNotFound) {
			h.responseJson(w, r, 400, fmt.Errorf("already exist"))
			return
		}
		h.responseJson(w, r, 500, err)
		return
	}

	now := time.Now().Unix()
	cache.CreatedAt = now
	cache.UsedAt = now
	if err := h.db.Insert(bolthold.NextSequence(), cache); err != nil {
		h.responseJson(w, r, 500, err)
		return
	}
	h.responseJson(w, r, 200, map[string]any{
		"cacheId": cache.ID,
	})
	return
}

// PATCH /_apis/artifactcache/caches/:id
func (h *Handler) upload(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJson(w, r, 400, err)
		return
	}

	cache := &Cache{}
	if err := h.db.Get(id, cache); err != nil {
		if errors.Is(err, bolthold.ErrNotFound) {
			h.responseJson(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
			return
		}
		h.responseJson(w, r, 500, err)
		return
	}

	if cache.Complete {
		h.responseJson(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}
	start, _, err := parseContentRange(r.Header.Get("Content-Range"))
	if err != nil {
		h.responseJson(w, r, 400, err)
		return
	}
	if err := h.storage.Write(cache.ID, start, r.Body); err != nil {
		h.responseJson(w, r, 500, err)
	}
	h.useCache(id)
	h.responseJson(w, r, 200)
}

// POST /_apis/artifactcache/caches/:id
func (h *Handler) commit(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJson(w, r, 400, err)
		return
	}

	cache := &Cache{}
	if err := h.db.Get(id, cache); err != nil {
		if errors.Is(err, bolthold.ErrNotFound) {
			h.responseJson(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
			return
		}
		h.responseJson(w, r, 500, err)
		return
	}

	if cache.Complete {
		h.responseJson(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}

	if err := h.storage.Commit(cache.ID, cache.Size); err != nil {
		h.responseJson(w, r, 500, err)
		return
	}

	cache.Complete = true
	if err := h.db.Update(cache.ID, cache); err != nil {
		h.responseJson(w, r, 500, err)
		return
	}

	h.responseJson(w, r, 200)
}

// GET /_apis/artifactcache/artifacts/:id
func (h *Handler) get(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJson(w, r, 400, err)
		return
	}
	h.useCache(id)
	h.storage.Serve(w, r, id)
}

// POST /_apis/artifactcache/clean
func (h *Handler) clean(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	// TODO: don't support force deleting cache entries
	// see: https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows#force-deleting-cache-entries

	h.responseJson(w, r, 200)
}

func (h *Handler) middleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		h.logger.Printf("%s %s", r.Method, r.RequestURI)
		handler(w, r, params)
		go h.gcCache()
	}
}

// if not found, return (nil, nil) instead of an error.
func (h *Handler) findCache(ctx context.Context, keys []string, version string) (*Cache, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	key := keys[0] // the first key is for exact match.

	cache := &Cache{
		Key:     key,
		Version: version,
	}
	cache.FillKeyVersionHash()

	if err := h.db.FindOne(cache, bolthold.Where("KeyVersionHash").Eq(cache.KeyVersionHash)); err != nil {
		if !errors.Is(err, bolthold.ErrNotFound) {
			return nil, err
		}
	} else if cache.Complete {
		return cache, nil
	}
	stop := fmt.Errorf("stop")

	for _, prefix := range keys[1:] {
		found := false
		if err := h.db.ForEach(bolthold.Where("Key").Ge(prefix).And("Version").Eq(version), func(v *Cache) error {
			if !strings.HasPrefix(v.Key, prefix) {
				return stop
			}
			if v.Complete {
				cache = v
				found = true
				return stop
			}
			return nil
		}); err != nil {
			if !errors.Is(err, stop) {
				return nil, err
			}
		}
		if found {
			return cache, nil
		}
	}
	return nil, nil
}

func (h *Handler) useCache(id int64) {
	cache := &Cache{}
	if err := h.db.Get(id, cache); err != nil {
		return
	}
	cache.UsedAt = time.Now().Unix()
	_ = h.db.Update(cache.ID, cache)
}

func (h *Handler) gcCache() {
	if h.gc.Load() {
		return
	}
	if !h.gc.CompareAndSwap(false, true) {
		return
	}
	defer h.gc.Store(false)

	if time.Since(h.gcAt) < time.Hour {
		h.logger.Infof("skip gc: %v", h.gcAt.String())
		return
	}
	h.gcAt = time.Now()
	h.logger.Infof("gc: %v", h.gcAt.String())

	const (
		keepUsed   = 30 * 24 * time.Hour
		keepUnused = 7 * 24 * time.Hour
		keepTemp   = 5 * time.Minute
	)

	var caches []*Cache
	if err := h.db.Find(&caches, bolthold.Where("UsedAt").Lt(time.Now().Add(-keepTemp).Unix())); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			if cache.Complete {
				continue
			}
			h.storage.Remove(cache.ID)
			if err := h.db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}

	caches = caches[:0]
	if err := h.db.Find(&caches, bolthold.Where("UsedAt").Lt(time.Now().Add(-keepUnused).Unix())); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := h.db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}

	caches = caches[:0]
	if err := h.db.Find(&caches, bolthold.Where("CreatedAt").Lt(time.Now().Add(-keepUsed).Unix())); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := h.db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}
}

func (h *Handler) responseJson(w http.ResponseWriter, r *http.Request, code int, v ...any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var data []byte
	if len(v) == 0 || v[0] == nil {
		data, _ = json.Marshal(struct{}{})
	} else if err, ok := v[0].(error); ok {
		h.logger.Errorf("%v %v: %v", r.Method, r.RequestURI, err)
		data, _ = json.Marshal(map[string]any{
			"error": err.Error(),
		})
	} else {
		data, _ = json.Marshal(v[0])
	}
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
