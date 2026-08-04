// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/require"
)

// newCapturingLogger returns a logger that writes JSON log records into the
// returned buffer, so tests can assert on emitted levels, messages, and fields.
func newCapturingLogger(t *testing.T) (*mlog.Logger, *mlog.Buffer) {
	t.Helper()
	logger, err := mlog.NewLogger()
	require.NoError(t, err)

	buf := &mlog.Buffer{}
	require.NoError(t, mlog.AddWriterTarget(logger, buf, true, mlog.StdAll...))

	return logger, buf
}

// flushLogs flushes the logger and returns the captured records, one map per
// emitted log line.
func flushLogs(t *testing.T, logger *mlog.Logger, buf *mlog.Buffer) []map[string]any {
	t.Helper()
	require.NoError(t, logger.Flush())

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		records = append(records, record)
	}
	return records
}
