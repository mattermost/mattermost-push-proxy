package server

import (
	"encoding/json"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

type logOptionsTest map[string]string

func TestEscapeDoubleQuotesOK(t *testing.T) {
	assert.Equal(t, "test", escapeDoubleQuotes("test"))
	assert.Equal(t, "\\\"test\\\"", escapeDoubleQuotes("\"test\""))
	assert.Equal(t, "\\\"\\\"test\\\"", escapeDoubleQuotes("\"\"test\""))
	assert.Equal(t, "\\\"\\\"\\\"\\\"", escapeDoubleQuotes("\"\"\"\""))
}

func TestDefaultLoggingConsoleLogConfigOK(t *testing.T) {
	var mapCfgEscaped mlog.LoggerConfiguration
	err := json.Unmarshal([]byte(defaultLoggingConsoleLogConfig()), &mapCfgEscaped)
	assert.NoError(t, err)

	var logOptionsConsole logOptionsTest

	targetCfg := mapCfgEscaped["def"]

	err = json.Unmarshal(targetCfg.Options, &logOptionsConsole)

	assert.NoError(t, err)
	assert.Equal(t, "console", targetCfg.Type)
	assert.Equal(t, "plain", targetCfg.Format)
	assert.Equal(t, "stdout", logOptionsConsole["out"])
}

func TestDefaultLoggingFileLogConfigOK(t *testing.T) {
	filename := randomString(10)
	var mapCfgEscaped mlog.LoggerConfiguration
	err := json.Unmarshal([]byte(defaultLoggingFileLogConfig(filename)), &mapCfgEscaped)
	assert.NoError(t, err)

	var logOptionsConsole logOptionsTest

	targetCfg := mapCfgEscaped["def"]

	err = json.Unmarshal(targetCfg.Options, &logOptionsConsole)

	assert.NoError(t, err)
	assert.Equal(t, "file", targetCfg.Type)
	assert.Equal(t, "plain", targetCfg.Format)
	assert.Equal(t, filename, logOptionsConsole["filename"])
}

func TestDefaultLoggingConfigOK(t *testing.T) {
	cfg := &ConfigPushProxy{
		EnableFileLog: false,
	}
	assert.Equal(t, defaultLoggingConsoleLogConfig(), DefaultLoggingConfig(cfg))

	cfg = &ConfigPushProxy{
		EnableFileLog:   true,
		LogFileLocation: randomString(10),
	}
	assert.Equal(t, defaultLoggingFileLogConfig(cfg.LogFileLocation), DefaultLoggingConfig(cfg))
}

func randomString(length int) string {
	// Define the character set
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\""
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate the string
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(result)
}
