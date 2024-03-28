package artifactcache

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timshannon/bolthold"
	"go.etcd.io/bbolt"
)

func TestHandler(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "artifactcache")
	handler, err := StartHandler(dir, "", 0, nil)
	require.NoError(t, err)

	base := fmt.Sprintf("%s%s", handler.ExternalURL(), urlBase)

	defer func() {
		t.Run("inpect db", func(t *testing.T) {
			db, err := handler.openDB()
			require.NoError(t, err)
			defer db.Close()
			require.NoError(t, db.Bolt().View(func(tx *bbolt.Tx) error {
				return tx.Bucket([]byte("Cache")).ForEach(func(k, v []byte) error {
					t.Logf("%s: %s", k, v)
					return nil
				})
			}))
		})
		t.Run("close", func(t *testing.T) {
			require.NoError(t, handler.Close())
			assert.Nil(t, handler.server)
			assert.Nil(t, handler.listener)
			_, err := http.Post(fmt.Sprintf("%s/caches/%d", base, 1), "", nil)
			assert.Error(t, err)
		})
	}()

	t.Run("get not exist", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, key, version))
		require.NoError(t, err)
		require.Equal(t, 204, resp.StatusCode)
	})

	t.Run("reserve and upload", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		uploadCacheNormally(t, base, key, version, content)
	})

	t.Run("clean", func(t *testing.T) {
		resp, err := http.Post(fmt.Sprintf("%s/clean", base), "", nil)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("reserve with bad request", func(t *testing.T) {
		body := []byte(`invalid json`)
		require.NoError(t, err)
		resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("duplicate reserve", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		var first, second struct {
			CacheID uint64 `json:"cacheId"`
		}
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			require.NoError(t, json.NewDecoder(resp.Body).Decode(&first))
			assert.NotZero(t, first.CacheID)
		}
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			require.NoError(t, json.NewDecoder(resp.Body).Decode(&second))
			assert.NotZero(t, second.CacheID)
		}

		assert.NotEqual(t, first.CacheID, second.CacheID)
	})

	t.Run("upload with bad id", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch,
			fmt.Sprintf("%s/caches/invalid_id", base), bytes.NewReader(nil))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", "bytes 0-99/*")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("upload without reserve", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch,
			fmt.Sprintf("%s/caches/%d", base, 1000), bytes.NewReader(nil))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", "bytes 0-99/*")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("upload with complete", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		var id uint64
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			got := struct {
				CacheID uint64 `json:"cacheId"`
			}{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			id = got.CacheID
		}
		{
			req, err := http.NewRequest(http.MethodPatch,
				fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Range", "bytes 0-99/*")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, id), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
		{
			req, err := http.NewRequest(http.MethodPatch,
				fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Range", "bytes 0-99/*")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode)
		}
	})

	t.Run("upload with invalid range", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		var id uint64
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			got := struct {
				CacheID uint64 `json:"cacheId"`
			}{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			id = got.CacheID
		}
		{
			req, err := http.NewRequest(http.MethodPatch,
				fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Range", "bytes xx-99/*")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode)
		}
	})

	t.Run("commit with bad id", func(t *testing.T) {
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/invalid_id", base), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode)
		}
	})

	t.Run("commit with not exist id", func(t *testing.T) {
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, 100), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode)
		}
	})

	t.Run("duplicate commit", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		var id uint64
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			got := struct {
				CacheID uint64 `json:"cacheId"`
			}{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			id = got.CacheID
		}
		{
			req, err := http.NewRequest(http.MethodPatch,
				fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Range", "bytes 0-99/*")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, id), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, id), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 400, resp.StatusCode)
		}
	})

	t.Run("commit early", func(t *testing.T) {
		key := strings.ToLower(t.Name())
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		var id uint64
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		{
			body, err := json.Marshal(&Request{
				Key:     key,
				Version: version,
				Size:    100,
			})
			require.NoError(t, err)
			resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			got := struct {
				CacheID uint64 `json:"cacheId"`
			}{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			id = got.CacheID
		}
		{
			req, err := http.NewRequest(http.MethodPatch,
				fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content[:50]))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/octet-stream")
			req.Header.Set("Content-Range", "bytes 0-59/*")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
		{
			resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, id), "", nil)
			require.NoError(t, err)
			assert.Equal(t, 500, resp.StatusCode)
		}
	})

	t.Run("get with bad id", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/artifacts/invalid_id", base))
		require.NoError(t, err)
		require.Equal(t, 400, resp.StatusCode)
	})

	t.Run("get with not exist id", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/artifacts/%d", base, 100))
		require.NoError(t, err)
		require.Equal(t, 404, resp.StatusCode)
	})

	t.Run("get with not exist id", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/artifacts/%d", base, 100))
		require.NoError(t, err)
		require.Equal(t, 404, resp.StatusCode)
	})

	t.Run("get with multiple keys", func(t *testing.T) {
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		key := strings.ToLower(t.Name())
		keys := [3]string{
			key + "_a_b_c",
			key + "_a_b",
			key + "_a",
		}
		contents := [3][]byte{
			make([]byte, 100),
			make([]byte, 200),
			make([]byte, 300),
		}
		for i := range contents {
			_, err := rand.Read(contents[i])
			require.NoError(t, err)
			uploadCacheNormally(t, base, keys[i], version, contents[i])
			time.Sleep(time.Second) // ensure CreatedAt of caches are different
		}

		reqKeys := strings.Join([]string{
			key + "_a_b_x",
			key + "_a_b",
			key + "_a",
		}, ",")

		resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, reqKeys, version))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		/*
			Expect `key_a_b` because:
			- `key_a_b_x" doesn't match any caches.
			- `key_a_b" matches `key_a_b` and `key_a_b_c`, but `key_a_b` is newer.
		*/
		except := 1

		got := struct {
			Result          string `json:"result"`
			ArchiveLocation string `json:"archiveLocation"`
			CacheKey        string `json:"cacheKey"`
		}{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, "hit", got.Result)
		assert.Equal(t, keys[except], got.CacheKey)

		contentResp, err := http.Get(got.ArchiveLocation)
		require.NoError(t, err)
		require.Equal(t, 200, contentResp.StatusCode)
		content, err := io.ReadAll(contentResp.Body)
		require.NoError(t, err)
		assert.Equal(t, contents[except], content)
	})

	t.Run("case insensitive", func(t *testing.T) {
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		key := strings.ToLower(t.Name())
		content := make([]byte, 100)
		_, err := rand.Read(content)
		require.NoError(t, err)
		uploadCacheNormally(t, base, key+"_ABC", version, content)

		{
			reqKey := key + "_aBc"
			resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, reqKey, version))
			require.NoError(t, err)
			require.Equal(t, 200, resp.StatusCode)
			got := struct {
				Result          string `json:"result"`
				ArchiveLocation string `json:"archiveLocation"`
				CacheKey        string `json:"cacheKey"`
			}{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			assert.Equal(t, "hit", got.Result)
			assert.Equal(t, key+"_abc", got.CacheKey)
		}
	})

	t.Run("exact keys are prefered (key 0)", func(t *testing.T) {
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		key := strings.ToLower(t.Name())
		keys := [3]string{
			key + "_a",
			key + "_a_b_c",
			key + "_a_b",
		}
		contents := [3][]byte{
			make([]byte, 100),
			make([]byte, 200),
			make([]byte, 300),
		}
		for i := range contents {
			_, err := rand.Read(contents[i])
			require.NoError(t, err)
			uploadCacheNormally(t, base, keys[i], version, contents[i])
			time.Sleep(time.Second) // ensure CreatedAt of caches are different
		}

		reqKeys := strings.Join([]string{
			key + "_a",
			key + "_a_b",
		}, ",")

		resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, reqKeys, version))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		/*
			Expect `key_a` because:
			- `key_a` matches `key_a`, `key_a_b` and `key_a_b_c`, but `key_a` is an exact match.
			- `key_a_b` matches `key_a_b` and `key_a_b_c`, but previous key had a match
		*/
		expect := 0

		got := struct {
			ArchiveLocation string `json:"archiveLocation"`
			CacheKey        string `json:"cacheKey"`
		}{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, keys[expect], got.CacheKey)

		contentResp, err := http.Get(got.ArchiveLocation)
		require.NoError(t, err)
		require.Equal(t, 200, contentResp.StatusCode)
		content, err := io.ReadAll(contentResp.Body)
		require.NoError(t, err)
		assert.Equal(t, contents[expect], content)
	})

	t.Run("exact keys are prefered (key 1)", func(t *testing.T) {
		version := "c19da02a2bd7e77277f1ac29ab45c09b7d46a4ee758284e26bb3045ad11d9d20"
		key := strings.ToLower(t.Name())
		keys := [3]string{
			key + "_a",
			key + "_a_b_c",
			key + "_a_b",
		}
		contents := [3][]byte{
			make([]byte, 100),
			make([]byte, 200),
			make([]byte, 300),
		}
		for i := range contents {
			_, err := rand.Read(contents[i])
			require.NoError(t, err)
			uploadCacheNormally(t, base, keys[i], version, contents[i])
			time.Sleep(time.Second) // ensure CreatedAt of caches are different
		}

		reqKeys := strings.Join([]string{
			"------------------------------------------------------",
			key + "_a",
			key + "_a_b",
		}, ",")

		resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, reqKeys, version))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)

		/*
			Expect `key_a` because:
			- `------------------------------------------------------` doesn't match any caches.
			- `key_a` matches `key_a`, `key_a_b` and `key_a_b_c`, but `key_a` is an exact match.
			- `key_a_b` matches `key_a_b` and `key_a_b_c`, but previous key had a match
		*/
		expect := 0

		got := struct {
			ArchiveLocation string `json:"archiveLocation"`
			CacheKey        string `json:"cacheKey"`
		}{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, keys[expect], got.CacheKey)

		contentResp, err := http.Get(got.ArchiveLocation)
		require.NoError(t, err)
		require.Equal(t, 200, contentResp.StatusCode)
		content, err := io.ReadAll(contentResp.Body)
		require.NoError(t, err)
		assert.Equal(t, contents[expect], content)
	})
}

func uploadCacheNormally(t *testing.T, base, key, version string, content []byte) {
	var id uint64
	{
		body, err := json.Marshal(&Request{
			Key:     key,
			Version: version,
			Size:    int64(len(content)),
		})
		require.NoError(t, err)
		resp, err := http.Post(fmt.Sprintf("%s/caches", base), "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		got := struct {
			CacheID uint64 `json:"cacheId"`
		}{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		id = got.CacheID
	}
	{
		req, err := http.NewRequest(http.MethodPatch,
			fmt.Sprintf("%s/caches/%d", base, id), bytes.NewReader(content))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", "bytes 0-99/*")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	}
	{
		resp, err := http.Post(fmt.Sprintf("%s/caches/%d", base, id), "", nil)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	}
	var archiveLocation string
	{
		resp, err := http.Get(fmt.Sprintf("%s/cache?keys=%s&version=%s", base, key, version))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		got := struct {
			Result          string `json:"result"`
			ArchiveLocation string `json:"archiveLocation"`
			CacheKey        string `json:"cacheKey"`
		}{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, "hit", got.Result)
		assert.Equal(t, strings.ToLower(key), got.CacheKey)
		archiveLocation = got.ArchiveLocation
	}
	{
		resp, err := http.Get(archiveLocation) //nolint:gosec
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	}
}

func TestHandler_gcCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "artifactcache")
	handler, err := StartHandler(dir, "", 0, nil)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, handler.Close())
	}()

	now := time.Now()

	cases := []struct {
		Cache *Cache
		Kept  bool
	}{
		{
			// should be kept, since it's used recently and not too old.
			Cache: &Cache{
				Key:       "test_key_1",
				Version:   "test_version",
				Complete:  true,
				UsedAt:    now.Unix(),
				CreatedAt: now.Add(-time.Hour).Unix(),
			},
			Kept: true,
		},
		{
			// should be removed, since it's not complete and not used for a while.
			Cache: &Cache{
				Key:       "test_key_2",
				Version:   "test_version",
				Complete:  false,
				UsedAt:    now.Add(-(keepTemp + time.Second)).Unix(),
				CreatedAt: now.Add(-(keepTemp + time.Hour)).Unix(),
			},
			Kept: false,
		},
		{
			// should be removed, since it's not used for a while.
			Cache: &Cache{
				Key:       "test_key_3",
				Version:   "test_version",
				Complete:  true,
				UsedAt:    now.Add(-(keepUnused + time.Second)).Unix(),
				CreatedAt: now.Add(-(keepUnused + time.Hour)).Unix(),
			},
			Kept: false,
		},
		{
			// should be removed, since it's used but too old.
			Cache: &Cache{
				Key:       "test_key_3",
				Version:   "test_version",
				Complete:  true,
				UsedAt:    now.Unix(),
				CreatedAt: now.Add(-(keepUsed + time.Second)).Unix(),
			},
			Kept: false,
		},
		{
			// should be kept, since it has a newer edition but be used recently.
			Cache: &Cache{
				Key:       "test_key_1",
				Version:   "test_version",
				Complete:  true,
				UsedAt:    now.Add(-(keepOld - time.Minute)).Unix(),
				CreatedAt: now.Add(-(time.Hour + time.Second)).Unix(),
			},
			Kept: true,
		},
		{
			// should be removed, since it has a newer edition and not be used recently.
			Cache: &Cache{
				Key:       "test_key_1",
				Version:   "test_version",
				Complete:  true,
				UsedAt:    now.Add(-(keepOld + time.Second)).Unix(),
				CreatedAt: now.Add(-(time.Hour + time.Second)).Unix(),
			},
			Kept: false,
		},
	}

	db, err := handler.openDB()
	require.NoError(t, err)
	for _, c := range cases {
		require.NoError(t, insertCache(db, c.Cache))
	}
	require.NoError(t, db.Close())

	handler.gcAt = time.Time{} // ensure gcCache will not skip
	handler.gcCache()

	db, err = handler.openDB()
	require.NoError(t, err)
	for i, v := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, v.Cache.Key), func(t *testing.T) {
			cache := &Cache{}
			err = db.Get(v.Cache.ID, cache)
			if v.Kept {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, bolthold.ErrNotFound)
			}
		})
	}
	require.NoError(t, db.Close())
}
