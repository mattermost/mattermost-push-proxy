// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"os"
	"testing"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLevelsFor(t *testing.T) {
	contains := func(levels []mlog.Level, target mlog.Level) bool {
		for _, l := range levels {
			if l.ID == target.ID {
				return true
			}
		}
		return false
	}

	// Panic/Fatal/Error are always emitted regardless of the configured level.
	for _, level := range []string{"error", "warn", "info", "debug", "", "garbage"} {
		levels := levelsFor(level)
		assert.True(t, contains(levels, mlog.LvlError), "%q should always include Error", level)
		assert.True(t, contains(levels, mlog.LvlFatal), "%q should always include Fatal", level)
	}

	t.Run("error", func(t *testing.T) {
		levels := levelsFor("error")
		assert.False(t, contains(levels, mlog.LvlWarn))
		assert.False(t, contains(levels, mlog.LvlInfo))
		assert.False(t, contains(levels, mlog.LvlDebug))
	})

	t.Run("warn", func(t *testing.T) {
		levels := levelsFor("warn")
		assert.True(t, contains(levels, mlog.LvlWarn))
		assert.False(t, contains(levels, mlog.LvlInfo))
		assert.False(t, contains(levels, mlog.LvlDebug))
	})

	t.Run("info excludes debug and trace", func(t *testing.T) {
		levels := levelsFor("info")
		assert.True(t, contains(levels, mlog.LvlWarn))
		assert.True(t, contains(levels, mlog.LvlInfo))
		assert.False(t, contains(levels, mlog.LvlDebug))
		assert.False(t, contains(levels, mlog.LvlTrace))
	})

	t.Run("unknown and empty default to info", func(t *testing.T) {
		for _, level := range []string{"", "garbage"} {
			levels := levelsFor(level)
			assert.True(t, contains(levels, mlog.LvlInfo), "%q should include Info", level)
			assert.False(t, contains(levels, mlog.LvlDebug), "%q should not include Debug", level)
		}
	})

	t.Run("debug includes debug and trace", func(t *testing.T) {
		levels := levelsFor("debug")
		assert.True(t, contains(levels, mlog.LvlDebug))
		assert.True(t, contains(levels, mlog.LvlTrace))
	})

	t.Run("case insensitive", func(t *testing.T) {
		levels := levelsFor("DEBUG")
		assert.True(t, contains(levels, mlog.LvlDebug))
	})
}
