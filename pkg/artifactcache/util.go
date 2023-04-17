package artifactcache

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"xorm.io/xorm"
)

func responseJson(w http.ResponseWriter, r *http.Request, code int, v ...any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var data []byte
	if len(v) == 0 || v[0] == nil {
		data, _ = json.Marshal(struct{}{})
	} else if err, ok := v[0].(error); ok {
		logger.Errorf("%v %v: %v", r.Method, r.RequestURI, err)
		data, _ = json.Marshal(map[string]any{
			"error": err.Error(),
		})
	} else {
		data, _ = json.Marshal(v[0])
	}
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

func getOutboundIP() (net.IP, error) {
	// FIXME: It makes more sense to use the gateway IP address of container network
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP, nil
	}
	if ifaces, err := net.Interfaces(); err == nil {
		for _, i := range ifaces {
			if addrs, err := i.Addrs(); err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip.IsGlobalUnicast() {
						return ip, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("no outbound IP address found")
}

// engine is a wrapper of *xorm.Engine, with a lock.
// To avoid racing of sqlite, we don't care performance here.
type engine struct {
	e *xorm.Engine
	m sync.Mutex
}

func (e *engine) Exec(f func(*xorm.Session) error) error {
	e.m.Lock()
	defer e.m.Unlock()

	sess := e.e.NewSession()
	defer sess.Close()

	return f(sess)
}

func (e *engine) ExecBool(f func(*xorm.Session) (bool, error)) (bool, error) {
	e.m.Lock()
	defer e.m.Unlock()

	sess := e.e.NewSession()
	defer sess.Close()

	return f(sess)
}
