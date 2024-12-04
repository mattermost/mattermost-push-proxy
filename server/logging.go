package server

import (
	"fmt"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"strings"
)

func NewMlogLogger(cfg *ConfigPushProxy) (*mlog.Logger, error) {
	// Initialize the logger - begin
	logger, err := mlog.NewLogger()
	if err != nil {
		return nil, err
	}
	cfgJSON := cfg.LoggingCfgJSON
	if cfg.LoggingCfgFile == "" && cfgJSON == "" {
		// if no logging defined, use default config (console output)
		cfgJSON = defaultLoggingConfig(cfg)
	}
	err = logger.Configure(cfg.LoggingCfgFile, cfgJSON, nil)
	if err != nil {
		return logger, err
	}

	return logger, nil
}

func defaultLoggingConfig(cfg *ConfigPushProxy) string {
	if !cfg.EnableFileLog && !cfg.EnableConsoleLog {
		return defaultLoggingConsoleLogConfig()
	} else if cfg.EnableFileLog && !cfg.EnableConsoleLog {
		if cfg.LogFileLocation == "" {
			return defaultLoggingConsoleLogConfig()
		}
		return defaultLoggingFileLogConfig(cfg.LogFileLocation)
	} else if !cfg.EnableFileLog && cfg.EnableConsoleLog {
		return defaultLoggingConsoleLogConfig()
	} else {
		if cfg.LogFileLocation == "" {
			return defaultLoggingConsoleLogConfig()
		}
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
