package artifactcache

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	urlBase = "/_apis/artifactcache"
)

var logger = log.StandardLogger().WithField("module", "cache_request")

type Handler struct {
	engine   engine
	storage  *Storage
	router   *chi.Mux
	listener net.Listener

	gc   atomic.Bool
	gcAt time.Time

	outboundIP string
}

func StartHandler(dir, outboundIP string, port uint16) (*Handler, error) {
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

	e, err := xorm.NewEngine("sqlite", filepath.Join(dir, "sqlite.db"))
	if err != nil {
		return nil, err
	}
	if err := e.Sync(&Cache{}); err != nil {
		return nil, err
	}
	h.engine = engine{e: e}

	storage, err := NewStorage(filepath.Join(dir, "cache"))
	if err != nil {
		return nil, err
	}
	h.storage = storage

	if outboundIP != "" {
		h.outboundIP = outboundIP
	} else if ip, err := getOutboundIP(); err != nil {
		return nil, err
	} else {
		h.outboundIP = ip.String()
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: logger}))
	router.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
			go h.gcCache()
		})
	})
	router.Use(middleware.Logger)
	router.Route(urlBase, func(r chi.Router) {
		r.Get("/cache", h.find)
		r.Route("/caches", func(r chi.Router) {
			r.Post("/", h.reserve)
			r.Route("/{id}", func(r chi.Router) {
				r.Patch("/", h.upload)
				r.Post("/", h.commit)
			})
		})
		r.Get("/artifacts/{id}", h.get)
		r.Post("/clean", h.clean)
	})

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
func (h *Handler) find(w http.ResponseWriter, r *http.Request) {
	keys := strings.Split(r.URL.Query().Get("keys"), ",")
	version := r.URL.Query().Get("version")

	cache, err := h.findCache(r.Context(), keys, version)
	if err != nil {
		responseJson(w, r, 500, err)
		return
	}
	if cache == nil {
		responseJson(w, r, 204)
		return
	}

	if ok, err := h.storage.Exist(cache.ID); err != nil {
		responseJson(w, r, 500, err)
		return
	} else if !ok {
		_ = h.engine.Exec(func(sess *xorm.Session) error {
			_, err := sess.Delete(cache)
			return err
		})
		responseJson(w, r, 204)
		return
	}
	responseJson(w, r, 200, map[string]any{
		"result":          "hit",
		"archiveLocation": fmt.Sprintf("%s%s/artifacts/%d", h.ExternalURL(), urlBase, cache.ID),
		"cacheKey":        cache.Key,
	})
}

// POST /_apis/artifactcache/caches
func (h *Handler) reserve(w http.ResponseWriter, r *http.Request) {
	cache := &Cache{}
	if err := render.Bind(r, cache); err != nil {
		responseJson(w, r, 400, err)
		return
	}

	if ok, err := h.engine.ExecBool(func(sess *xorm.Session) (bool, error) {
		return sess.Where(builder.Eq{"key": cache.Key, "version": cache.Version}).Get(&Cache{})
	}); err != nil {
		responseJson(w, r, 500, err)
		return
	} else if ok {
		responseJson(w, r, 400, fmt.Errorf("already exist"))
		return
	}

	if err := h.engine.Exec(func(sess *xorm.Session) error {
		_, err := sess.Insert(cache)
		return err
	}); err != nil {
		responseJson(w, r, 500, err)
		return
	}
	responseJson(w, r, 200, map[string]any{
		"cacheId": cache.ID,
	})
	return
}

// PATCH /_apis/artifactcache/caches/:id
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		responseJson(w, r, 400, err)
		return
	}

	cache := &Cache{
		ID: id,
	}

	if ok, err := h.engine.ExecBool(func(sess *xorm.Session) (bool, error) {
		return sess.Get(cache)
	}); err != nil {
		responseJson(w, r, 500, err)
		return
	} else if !ok {
		responseJson(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
		return
	}

	if cache.Complete {
		responseJson(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}
	start, _, err := parseContentRange(r.Header.Get("Content-Range"))
	if err != nil {
		responseJson(w, r, 400, err)
		return
	}
	if err := h.storage.Write(cache.ID, start, r.Body); err != nil {
		responseJson(w, r, 500, err)
	}
	h.useCache(r.Context(), id)
	responseJson(w, r, 200)
}

// POST /_apis/artifactcache/caches/:id
func (h *Handler) commit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		responseJson(w, r, 400, err)
		return
	}

	cache := &Cache{
		ID: id,
	}
	if ok, err := h.engine.ExecBool(func(sess *xorm.Session) (bool, error) {
		return sess.Get(cache)
	}); err != nil {
		responseJson(w, r, 500, err)
		return
	} else if !ok {
		responseJson(w, r, 400, fmt.Errorf("cache %d: not reserved", id))
		return
	}

	if cache.Complete {
		responseJson(w, r, 400, fmt.Errorf("cache %v %q: already complete", cache.ID, cache.Key))
		return
	}

	if err := h.storage.Commit(cache.ID, cache.Size); err != nil {
		responseJson(w, r, 500, err)
		return
	}

	cache.Complete = true
	if err := h.engine.Exec(func(sess *xorm.Session) error {
		_, err := sess.ID(cache.ID).Cols("complete").Update(cache)
		return err
	}); err != nil {
		responseJson(w, r, 500, err)
		return
	}

	responseJson(w, r, 200)
}

// GET /_apis/artifactcache/artifacts/:id
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		responseJson(w, r, 400, err)
		return
	}
	h.useCache(r.Context(), id)
	h.storage.Serve(w, r, id)
}

// POST /_apis/artifactcache/clean
func (h *Handler) clean(w http.ResponseWriter, r *http.Request) {
	// TODO: don't support force deleting cache entries
	// see: https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows#force-deleting-cache-entries

	responseJson(w, r, 200)
}

// if not found, return (nil, nil) instead of an error.
func (h *Handler) findCache(ctx context.Context, keys []string, version string) (*Cache, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	key := keys[0] // the first key is for exact match.

	cache := &Cache{}
	if ok, err := h.engine.ExecBool(func(sess *xorm.Session) (bool, error) {
		return sess.Where(builder.Eq{"key": key, "version": version, "complete": true}).Get(cache)
	}); err != nil {
		return nil, err
	} else if ok {
		return cache, nil
	}

	for _, prefix := range keys[1:] {
		if ok, err := h.engine.ExecBool(func(sess *xorm.Session) (bool, error) {
			return sess.Where(builder.And(
				builder.Like{"key", prefix + "%"},
				builder.Eq{"version": version, "complete": true},
			)).OrderBy("id DESC").Get(cache)
		}); err != nil {
			return nil, err
		} else if ok {
			return cache, nil
		}
	}
	return nil, nil
}

func (h *Handler) useCache(ctx context.Context, id int64) {
	// keep quiet
	_ = h.engine.Exec(func(sess *xorm.Session) error {
		_, err := sess.Context(ctx).Cols("used_at").Update(&Cache{
			ID:     id,
			UsedAt: time.Now().Unix(),
		})
		return err
	})
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
		logger.Infof("skip gc: %v", h.gcAt.String())
		return
	}
	h.gcAt = time.Now()
	logger.Infof("gc: %v", h.gcAt.String())

	const (
		keepUsed   = 30 * 24 * time.Hour
		keepUnused = 7 * 24 * time.Hour
		keepTemp   = 5 * time.Minute
	)

	var caches []*Cache
	if err := h.engine.Exec(func(sess *xorm.Session) error {
		return sess.Where(builder.And(builder.Lt{"used_at": time.Now().Add(-keepTemp).Unix()}, builder.Eq{"complete": false})).
			Find(&caches)
	}); err != nil {
		logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := h.engine.Exec(func(sess *xorm.Session) error {
				_, err := sess.Delete(cache)
				return err
			}); err != nil {
				logger.Warnf("delete cache: %v", err)
				continue
			}
			logger.Infof("deleted cache: %+v", cache)
		}
	}

	caches = caches[:0]
	if err := h.engine.Exec(func(sess *xorm.Session) error {
		return sess.Where(builder.Lt{"used_at": time.Now().Add(-keepUnused).Unix()}).
			Find(&caches)
	}); err != nil {
		logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := h.engine.Exec(func(sess *xorm.Session) error {
				_, err := sess.Delete(cache)
				return err
			}); err != nil {
				logger.Warnf("delete cache: %v", err)
				continue
			}
			logger.Infof("deleted cache: %+v", cache)
		}
	}

	caches = caches[:0]
	if err := h.engine.Exec(func(sess *xorm.Session) error {
		return sess.Where(builder.Lt{"created_at": time.Now().Add(-keepUsed).Unix()}).
			Find(&caches)
	}); err != nil {
		logger.Warnf("find caches: %v", err)
	} else {
		for _, cache := range caches {
			h.storage.Remove(cache.ID)
			if err := h.engine.Exec(func(sess *xorm.Session) error {
				_, err := sess.Delete(cache)
				return err
			}); err != nil {
				logger.Warnf("delete cache: %v", err)
				continue
			}
			logger.Infof("deleted cache: %+v", cache)
		}
	}
}
