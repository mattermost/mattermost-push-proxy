package server

import (
	"encoding/json"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

type logOptionsTest map[string]string

func TestEscapeDoubleQuotes(t *testing.T) {
	t.Run("No quotes", func(t *testing.T) {
		assert.Equal(t, "test", escapeDoubleQuotes("test"))
	})
	t.Run("One quote", func(t *testing.T) {
		assert.Equal(t, "\\\"test", escapeDoubleQuotes("\"test"))
	})
	t.Run("Two quotes", func(t *testing.T) {
		assert.Equal(t, "\\\"test\\\"", escapeDoubleQuotes("\"test\""))
	})
	t.Run("Three quotes", func(t *testing.T) {
		assert.Equal(t, "\\\"\\\"test\\\"", escapeDoubleQuotes("\"\"test\""))
	})
	t.Run("Four quotes", func(t *testing.T) {
		assert.Equal(t, "\\\"\\\"\\\"\\\"", escapeDoubleQuotes("\"\"\"\""))
	})
}

func TestDefaultLoggingConsoleLogConfig(t *testing.T) {
	var mapCfgEscaped = new(mlog.LoggerConfiguration)
	var logOptionsConsole = new(logOptionsTest)

	var targetCfg mlog.TargetCfg

	t.Run("Unmarshall base correctly", func(t *testing.T) {
		err := json.Unmarshal([]byte(defaultLoggingConsoleLogConfig()), mapCfgEscaped)
		require.NoError(t, err)
		require.NotNil(t, mapCfgEscaped)
		targetCfg = (*mapCfgEscaped)["def"]
		assert.Equal(t, "console", targetCfg.Type)
		assert.Equal(t, "plain", targetCfg.Format)
	})

	t.Run("Unmarshall options correctly", func(t *testing.T) {
		err := json.Unmarshal(targetCfg.Options, logOptionsConsole)
		require.NoError(t, err)
		require.NotNil(t, logOptionsConsole)
		assert.Equal(t, "stdout", (*logOptionsConsole)["out"])
	})
}

func TestDefaultLoggingFileLogConfig(t *testing.T) {
	filename := model.NewId()
	var mapCfgEscaped = new(mlog.LoggerConfiguration)
	var logOptionsConsole = new(logOptionsTest)

	var targetCfg mlog.TargetCfg

	t.Run("Unmarshall base correctly", func(t *testing.T) {
		err := json.Unmarshal([]byte(defaultLoggingFileLogConfig(filename)), mapCfgEscaped)
		require.NoError(t, err)
		require.NotNil(t, mapCfgEscaped)
		targetCfg = (*mapCfgEscaped)["def"]
		assert.Equal(t, "file", targetCfg.Type)
		assert.Equal(t, "plain", targetCfg.Format)
	})

	t.Run("Unmarshall options correctly", func(t *testing.T) {
		err := json.Unmarshal(targetCfg.Options, logOptionsConsole)
		require.NoError(t, err)
		require.NotNil(t, logOptionsConsole)
		assert.Equal(t, filename, (*logOptionsConsole)["filename"])
	})
}

func TestDefaultLoggingConfig(t *testing.T) {
	t.Run("Console config is get correctly", func(t *testing.T) {
		cfg := &ConfigPushProxy{
			EnableFileLog: false,
		}
		assert.Equal(t, defaultLoggingConsoleLogConfig(), defaultLoggingConfig(cfg))
	})

	t.Run("File config is get correctly", func(t *testing.T) {
		cfg := &ConfigPushProxy{
			EnableFileLog:   true,
			LogFileLocation: model.NewId(),
		}
		assert.Equal(t, defaultLoggingFileLogConfig(cfg.LogFileLocation), defaultLoggingConfig(cfg))
	})
}

func TestNewMlogLogger(t *testing.T) {
	t.Run("Instancing logger with console for legacy conf", func(t *testing.T) {
		cfg := &ConfigPushProxy{
			EnableFileLog: false,
		}
		logger, err := NewMlogLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("Instancing logger with file for legacy conf", func(t *testing.T) {
		log, err := os.CreateTemp("", "log")
		require.NoError(t, err)

		err = log.Close()
		require.NoError(t, err)
		defer os.Remove(log.Name())

		cfg := &ConfigPushProxy{
			EnableFileLog:   true,
			LogFileLocation: log.Name(),
		}

		logger, err := NewMlogLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("Instancing logger with file", func(t *testing.T) {
		conf, err := os.CreateTemp("", "logget-cfg-conf.json")
		require.NoError(t, err)

		_, err = conf.WriteString(defaultLoggingConsoleLogConfig())
		require.NoError(t, err)

		err = conf.Close()
		require.NoError(t, err)
		defer os.Remove(conf.Name())

		cfg := &ConfigPushProxy{
			LoggingCfgFile: conf.Name(),
		}

		logger, err := NewMlogLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("Instancing logger with json", func(t *testing.T) {
		conf, err := os.CreateTemp("", "logget-cfg-conf.json")
		require.NoError(t, err)

		_, err = conf.WriteString(defaultLoggingConsoleLogConfig())
		require.NoError(t, err)

		err = conf.Close()
		require.NoError(t, err)
		defer os.Remove(conf.Name())

		cfg := &ConfigPushProxy{
			LoggingCfgJSON: defaultLoggingConsoleLogConfig(),
		}

		logger, err := NewMlogLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
}
