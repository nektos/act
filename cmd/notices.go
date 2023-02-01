package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

type Notice struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func displayNotices(input *Input) {
	select {
	case notices := <-noticesLoaded:
		if len(notices) > 0 {
			noticeLogger := log.New()
			if input.jsonLogger {
				noticeLogger.SetFormatter(&log.JSONFormatter{})
			} else {
				noticeLogger.SetFormatter(&log.TextFormatter{
					DisableQuote:     true,
					DisableTimestamp: true,
					PadLevelText:     true,
				})
			}

			fmt.Printf("\n")
			for _, notice := range notices {
				level, err := log.ParseLevel(notice.Level)
				if err != nil {
					level = log.InfoLevel
				}
				noticeLogger.Log(level, notice.Message)
			}
		}
	case <-time.After(time.Second * 1):
		log.Debugf("Timeout waiting for notices")
	}
}

var noticesLoaded = make(chan []Notice)

func loadVersionNotices(version string) {
	go func() {
		noticesLoaded <- getVersionNotices(version)
	}()
}

const NoticeURL = "https://api.nektosact.com/notices"

func getVersionNotices(version string) []Notice {
	if os.Getenv("ACT_DISABLE_VERSION_CHECK") == "1" {
		return nil
	}

	noticeURL, err := url.Parse(NoticeURL)
	if err != nil {
		log.Error(err)
		return nil
	}
	query := noticeURL.Query()
	query.Add("os", runtime.GOOS)
	query.Add("arch", runtime.GOARCH)
	query.Add("version", version)

	noticeURL.RawQuery = query.Encode()

	client := &http.Client{}
	req, err := http.NewRequest("GET", noticeURL.String(), nil)
	if err != nil {
		log.Debug(err)
		return nil
	}

	etag := loadNoticesEtag()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Debug(err)
		return nil
	}

	newEtag := resp.Header.Get("ETag")
	if newEtag != "" {
		saveNoticesEtag(newEtag)
	}

	defer resp.Body.Close()
	notices := []Notice{}
	if err := json.NewDecoder(resp.Body).Decode(&notices); err != nil {
		log.Debug(err)
		return nil
	}

	return notices
}

func loadNoticesEtag() string {
	p := etagPath()
	content, err := ioutil.ReadFile(p)
	if err != nil {
		log.Debugf("Unable to load etag from %s: %e", p, err)
	}
	return strings.TrimSuffix(string(content), "\n")
}

func saveNoticesEtag(etag string) {
	p := etagPath()
	err := os.WriteFile(p, []byte(strings.TrimSuffix(etag, "\n")), 0600)
	if err != nil {
		log.Debugf("Unable to save etag to %s: %e", p, err)
	}
}

func etagPath() string {
	var xdgCache string
	var ok bool
	if xdgCache, ok = os.LookupEnv("XDG_CACHE_HOME"); !ok || xdgCache == "" {
		if home, err := homedir.Dir(); err == nil {
			xdgCache = filepath.Join(home, ".cache")
		} else if xdgCache, err = filepath.Abs("."); err != nil {
			log.Fatal(err)
		}
	}
	dir := filepath.Join(xdgCache, "act")
	if err := os.MkdirAll(dir, 0777); err != nil {
		log.Fatal(err)
	}
	return filepath.Join(dir, ".notices.etag")
}
