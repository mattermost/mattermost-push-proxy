package server

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestNewMlogLogger(t *testing.T) {
	t.Run("Instancing logger with implicit plain console", func(t *testing.T) {
		cfg := &ConfigPushProxy{
			EnableFileLog: false,
		}
		logger, err := NewLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("Instancing logger with json file", func(t *testing.T) {
		log, err := os.CreateTemp("", "log")
		require.NoError(t, err)

		err = log.Close()
		require.NoError(t, err)
		defer os.Remove(log.Name())

		cfg := &ConfigPushProxy{
			EnableFileLog:   true,
			LogFileLocation: log.Name(),
			LogFormat:       "json",
		}

		logger, err := NewLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})

	t.Run("Instancing logger with both file and console", func(t *testing.T) {
		log, err := os.CreateTemp("", "log")
		require.NoError(t, err)

		err = log.Close()
		require.NoError(t, err)
		defer os.Remove(log.Name())

		cfg := &ConfigPushProxy{
			EnableConsoleLog: true,
			EnableFileLog:    true,
			LogFileLocation:  log.Name(),
			LogFormat:        "json",
		}

		logger, err := NewLogger(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
}
