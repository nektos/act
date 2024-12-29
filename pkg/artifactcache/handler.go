package artifactcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	dir      string
	storage  *Storage
	router   *httprouter.Router
	listener net.Listener
	server   *http.Server
	logger   logrus.FieldLogger

	gcing atomic.Bool
	gcAt  time.Time

	outboundIP string
}

func StartHandler(dir, outboundIP string, port uint16, logger logrus.FieldLogger) (*Handler, error) {
	h := &Handler{}

	if logger == nil {
		discard := logrus.New()
		discard.Out = io.Discard
		logger = discard
	}
	logger = logger.WithField("module", "artifactcache")
	h.logger = logger

	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(home, ".cache", "actcache")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	h.dir = dir

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
	server := &http.Server{
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           router,
	}
	go func() {
		if err := server.Serve(listener); err != nil && errors.Is(err, net.ErrClosed) {
			logger.Errorf("http serve: %v", err)
		}
	}()
	h.listener = listener
	h.server = server

	return h, nil
}

func (h *Handler) ExternalURL() string {
	// TODO: make the external url configurable if necessary
	return fmt.Sprintf("http://%s:%d",
		h.outboundIP,
		h.listener.Addr().(*net.TCPAddr).Port)
}

func (h *Handler) Close() error {
	if h == nil {
		return nil
	}
	var retErr error
	if h.server != nil {
		err := h.server.Close()
		if err != nil {
			retErr = err
		}
		h.server = nil
	}
	if h.listener != nil {
		err := h.listener.Close()
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}
		if err != nil {
			retErr = err
		}
		h.listener = nil
	}
	return retErr
}

func (h *Handler) openDB() (*bolthold.Store, error) {
	return bolthold.Open(filepath.Join(h.dir, "bolt.db"), 0o644, &bolthold.Options{
		Encoder: json.Marshal,
		Decoder: json.Unmarshal,
		Options: &bbolt.Options{
			Timeout:      5 * time.Second,
			NoGrowSync:   bbolt.DefaultOptions.NoGrowSync,
			FreelistType: bbolt.DefaultOptions.FreelistType,
		},
	})
}

// GET /_apis/artifactcache/cache
func (h *Handler) find(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	keys := strings.Split(r.URL.Query().Get("keys"), ",")
	// cache keys are case insensitive
	for i, key := range keys {
		keys[i] = strings.ToLower(key)
	}
	version := r.URL.Query().Get("version")

	db, err := h.openDB()
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	defer db.Close()

	cache, err := findCache(db, keys, version)
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	if cache == nil {
		h.responseJSON(w, r, 204)
		return
	}

	if ok, err := h.storage.Exist(cache.ID); err != nil {
		h.responseJSON(w, r, 500, err)
		return
	} else if !ok {
		_ = db.Delete(cache.ID, cache)
		h.responseJSON(w, r, 204)
		return
	}
	h.responseJSON(w, r, 200, map[string]any{
		"result":          "hit",
		"archiveLocation": fmt.Sprintf("%s%s/artifacts/%d", h.ExternalURL(), urlBase, cache.ID),
		"cacheKey":        cache.Key,
	})
}

// POST /_apis/artifactcache/caches
func (h *Handler) reserve(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	api := &Request{}
	if err := json.NewDecoder(r.Body).Decode(api); err != nil {
		h.responseJSON(w, r, 400, err)
		return
	}
	// cache keys are case insensitive
	api.Key = strings.ToLower(api.Key)

	cache := api.ToCache()
	db, err := h.openDB()
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	defer db.Close()

	now := time.Now().Unix()
	cache.CreatedAt = now
	cache.UsedAt = now
	if err := insertCache(db, cache); err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	h.responseJSON(w, r, 200, map[string]any{
		"cacheId": cache.ID,
	})
}

// PATCH /_apis/artifactcache/caches/:id
func (h *Handler) upload(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseUint(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJSON(w, r, 400, err)
		return
	}

	cache := &Cache{}
	db, err := h.openDB()
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	defer db.Close()
	if err := db.Get(id, cache); err != nil {
		if errors.Is(err, bolthold.ErrNotFound) {
			h.responseJSON(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
			return
		}
		h.responseJSON(w, r, 500, err)
		return
	}

	if cache.Complete {
		h.responseJSON(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}
	db.Close()
	start, _, err := parseContentRange(r.Header.Get("Content-Range"))
	if err != nil {
		h.responseJSON(w, r, 400, err)
		return
	}
	if err := h.storage.Write(cache.ID, start, r.Body); err != nil {
		h.responseJSON(w, r, 500, err)
	}
	h.useCache(id)
	h.responseJSON(w, r, 200)
}

// POST /_apis/artifactcache/caches/:id
func (h *Handler) commit(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJSON(w, r, 400, err)
		return
	}

	cache := &Cache{}
	db, err := h.openDB()
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	defer db.Close()
	if err := db.Get(id, cache); err != nil {
		if errors.Is(err, bolthold.ErrNotFound) {
			h.responseJSON(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
			return
		}
		h.responseJSON(w, r, 500, err)
		return
	}

	if cache.Complete {
		h.responseJSON(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}

	db.Close()

	size, err := h.storage.Commit(cache.ID, cache.Size)
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	// write real size back to cache, it may be different from the current value when the request doesn't specify it.
	cache.Size = size

	db, err = h.openDB()
	if err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}
	defer db.Close()

	cache.Complete = true
	if err := db.Update(cache.ID, cache); err != nil {
		h.responseJSON(w, r, 500, err)
		return
	}

	h.responseJSON(w, r, 200)
}

// GET /_apis/artifactcache/artifacts/:id
func (h *Handler) get(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id, err := strconv.ParseUint(params.ByName("id"), 10, 64)
	if err != nil {
		h.responseJSON(w, r, 400, err)
		return
	}
	h.useCache(id)
	h.storage.Serve(w, r, id)
}

// POST /_apis/artifactcache/clean
func (h *Handler) clean(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// TODO: don't support force deleting cache entries
	// see: https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows#force-deleting-cache-entries

	h.responseJSON(w, r, 200)
}

func (h *Handler) middleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		h.logger.Debugf("%s %s", r.Method, r.RequestURI)
		handler(w, r, params)
		go h.gcCache()
	}
}

// if not found, return (nil, nil) instead of an error.
func findCache(db *bolthold.Store, keys []string, version string) (*Cache, error) {
	cache := &Cache{}
	for _, prefix := range keys {
		// if a key in the list matches exactly, don't return partial matches
		if err := db.FindOne(cache,
			bolthold.Where("Key").Eq(prefix).
				And("Version").Eq(version).
				And("Complete").Eq(true).
				SortBy("CreatedAt").Reverse()); err == nil || !errors.Is(err, bolthold.ErrNotFound) {
			if err != nil {
				return nil, fmt.Errorf("find cache: %w", err)
			}
			return cache, nil
		}
		prefixPattern := fmt.Sprintf("^%s", regexp.QuoteMeta(prefix))
		re, err := regexp.Compile(prefixPattern)
		if err != nil {
			continue
		}
		if err := db.FindOne(cache,
			bolthold.Where("Key").RegExp(re).
				And("Version").Eq(version).
				And("Complete").Eq(true).
				SortBy("CreatedAt").Reverse()); err != nil {
			if errors.Is(err, bolthold.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("find cache: %w", err)
		}
		return cache, nil
	}
	return nil, nil
}

func insertCache(db *bolthold.Store, cache *Cache) error {
	if err := db.Insert(bolthold.NextSequence(), cache); err != nil {
		return fmt.Errorf("insert cache: %w", err)
	}
	// write back id to db
	if err := db.Update(cache.ID, cache); err != nil {
		return fmt.Errorf("write back id to db: %w", err)
	}
	return nil
}

func (h *Handler) useCache(id uint64) {
	db, err := h.openDB()
	if err != nil {
		return
	}
	defer db.Close()
	cache := &Cache{}
	if err := db.Get(id, cache); err != nil {
		return
	}
	cache.UsedAt = time.Now().Unix()
	_ = db.Update(cache.ID, cache)
}

const (
	keepUsed   = 30 * 24 * time.Hour
	keepUnused = 7 * 24 * time.Hour
	keepTemp   = 5 * time.Minute
	keepOld    = 5 * time.Minute
)

func (h *Handler) gcCache() {
	if h.gcing.Load() {
		return
	}
	if !h.gcing.CompareAndSwap(false, true) {
		return
	}
	defer h.gcing.Store(false)

	if time.Since(h.gcAt) < time.Hour {
		h.logger.Debugf("skip gc: %v", h.gcAt.String())
		return
	}
	h.gcAt = time.Now()
	h.logger.Debugf("gc: %v", h.gcAt.String())

	db, err := h.openDB()
	if err != nil {
		return
	}
	defer db.Close()

	// Remove the caches which are not completed for a while, they are most likely to be broken.
	var caches []*Cache
	if err := db.Find(&caches, bolthold.
		Where("UsedAt").Lt(time.Now().Add(-keepTemp).Unix()).
		And("Complete").Eq(false),
	); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}

	// Remove the old caches which have not been used recently.
	caches = caches[:0]
	if err := db.Find(&caches, bolthold.
		Where("UsedAt").Lt(time.Now().Add(-keepUnused).Unix()),
	); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}

	// Remove the old caches which are too old.
	caches = caches[:0]
	if err := db.Find(&caches, bolthold.
		Where("CreatedAt").Lt(time.Now().Add(-keepUsed).Unix()),
	); err != nil {
		h.logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := db.Delete(cache.ID, cache); err != nil {
				h.logger.Warnf("delete cache: %v", err)
				continue
			}
			h.logger.Infof("deleted cache: %+v", cache)
		}
	}

	// Remove the old caches with the same key and version, keep the latest one.
	// Also keep the olds which have been used recently for a while in case of the cache is still in use.
	if results, err := db.FindAggregate(
		&Cache{},
		bolthold.Where("Complete").Eq(true),
		"Key", "Version",
	); err != nil {
		h.logger.Warnf("find aggregate caches: %v", err)
	} else {
		for _, result := range results {
			if result.Count() <= 1 {
				continue
			}
			result.Sort("CreatedAt")
			caches = caches[:0]
			result.Reduction(&caches)
			for _, cache := range caches[:len(caches)-1] {
				if time.Since(time.Unix(cache.UsedAt, 0)) < keepOld {
					// Keep it since it has been used recently, even if it's old.
					// Or it could break downloading in process.
					continue
				}
				h.storage.Remove(cache.ID)
				if err := db.Delete(cache.ID, cache); err != nil {
					h.logger.Warnf("delete cache: %v", err)
					continue
				}
				h.logger.Infof("deleted cache: %+v", cache)
			}
		}
	}
}

func (h *Handler) responseJSON(w http.ResponseWriter, r *http.Request, code int, v ...any) {
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

func parseContentRange(s string) (int64, int64, error) {
	// support the format like "bytes 11-22/*" only
	s, _, _ = strings.Cut(strings.TrimPrefix(s, "bytes "), "/")
	s1, s2, _ := strings.Cut(s, "-")

	start, err := strconv.ParseInt(s1, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse %q: %w", s, err)
	}
	stop, err := strconv.ParseInt(s2, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse %q: %w", s, err)
	}
	return start, stop, nil
}
