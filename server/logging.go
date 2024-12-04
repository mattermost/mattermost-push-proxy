package server

import (
	"fmt"
	"strings"
)

func DefaultLoggingConfig(cfg *ConfigPushProxy) string {
	if !cfg.EnableFileLog {
		return defaultLoggingConsoleLogConfig()
	} else {
		return defaultLoggingFileLogConfig(cfg.LogFileLocation)
	}
}

func defaultLoggingFileLogConfig(filename string) string {
	return fmt.Sprintf(`
		{
			"def": {
				"type": "file",
				"options": {
					"filename": "%s"
				},
				"format": "plain",
				"format_options": {
					"delim": " ",
					"min_level_len": 5,
					"min_msg_len": 40,
					"enable_color": true,
					"enable_caller": true
				},
				"levels": [
					{"id": 5, "name": "debug"},
					{"id": 4, "name": "info", "color": 36},
					{"id": 3, "name": "warn"},
					{"id": 2, "name": "error", "color": 31},
					{"id": 1, "name": "fatal", "stacktrace": true},
					{"id": 0, "name": "panic", "stacktrace": true}
				]
			}
		}`, escapeDoubleQuotes(filename))
}

func defaultLoggingConsoleLogConfig() string {
	return `
		{
			"def": {
				"type": "console",
				"options": {
					"out": "stdout"
				},
				"format": "plain",
				"format_options": {
					"delim": " ",
					"min_level_len": 5,
					"min_msg_len": 40,
					"enable_color": true,
					"enable_caller": true
				},
				"levels": [
					{"id": 5, "name": "debug"},
					{"id": 4, "name": "info", "color": 36},
					{"id": 3, "name": "warn"},
					{"id": 2, "name": "error", "color": 31},
					{"id": 1, "name": "fatal", "stacktrace": true},
					{"id": 0, "name": "panic", "stacktrace": true}
				]
			}
		}`
}

func escapeDoubleQuotes(input string) string {
	return strings.ReplaceAll(input, `"`, `\"`)
}
