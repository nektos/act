package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"

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

	resp, err := http.Get(noticeURL.String())
	if err != nil {
		log.Debug(err)
		return nil
	}

	defer resp.Body.Close()
	notices := []Notice{}
	if err := json.NewDecoder(resp.Body).Decode(&notices); err != nil {
		log.Debug(err)
		return nil
	}

	return notices
}
