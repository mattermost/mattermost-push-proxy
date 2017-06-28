package gobrake

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var defaultContextOnce sync.Once
var defaultContext map[string]interface{}

func getDefaultContext() map[string]interface{} {
	defaultContextOnce.Do(func() {
		defaultContext = map[string]interface{}{
			"notifier": map[string]interface{}{
				"name":    "gobrake",
				"version": "3.0.1",
				"url":     "https://github.com/airbrake/gobrake",
			},

			"language":     runtime.Version(),
			"os":           runtime.GOOS,
			"architecture": runtime.GOARCH,
		}
		if s, err := os.Hostname(); err == nil {
			defaultContext["hostname"] = s
		}
		if s := os.Getenv("GOPATH"); s != "" {
			list := filepath.SplitList(s)
			// TODO: multiple root dirs?
			defaultContext["rootDirectory"] = list[0]
		}
	})
	return defaultContext
}

type Error struct {
	Type      string       `json:"type"`
	Message   string       `json:"message"`
	Backtrace []StackFrame `json:"backtrace"`
}

type Notice struct {
	Errors  []Error                `json:"errors"`
	Context map[string]interface{} `json:"context"`
	Env     map[string]interface{} `json:"environment"`
	Session map[string]interface{} `json:"session"`
	Params  map[string]interface{} `json:"params"`
}

func (n *Notice) String() string {
	if len(n.Errors) == 0 {
		return "Notice<no errors>"
	}
	e := n.Errors[0]
	return fmt.Sprintf("Notice<%s: %s>", e.Type, e.Message)
}

func NewNotice(e interface{}, req *http.Request, depth int) *Notice {
	notice := &Notice{
		Errors: []Error{{
			Type:      fmt.Sprintf("%T", e),
			Message:   fmt.Sprint(e),
			Backtrace: stack(depth),
		}},
		Context: make(map[string]interface{}),
		Env:     make(map[string]interface{}),
		Session: make(map[string]interface{}),
		Params:  make(map[string]interface{}),
	}

	for k, v := range getDefaultContext() {
		notice.Context[k] = v
	}

	if req == nil {
		return notice
	}

	notice.Context["url"] = req.URL.String()
	notice.Context["httpMethod"] = req.Method
	if ua := req.Header.Get("User-Agent"); ua != "" {
		notice.Context["userAgent"] = ua
	}
	notice.Context["userAddr"] = remoteAddr(req)

	for k, v := range req.Header {
		if len(v) == 1 {
			notice.Env[k] = v[0]
		} else {
			notice.Env[k] = v
		}
	}

	return notice
}

func remoteAddr(req *http.Request) string {
	if s := req.Header.Get("X-Forwarded-For"); s != "" {
		parts := strings.Split(s, ",")
		return parts[0]
	}

	if s := req.Header.Get("X-Real-Ip"); s != "" {
		return s
	}

	parts := strings.Split(req.RemoteAddr, ":")
	return parts[0]
}
