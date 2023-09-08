// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerConsoleAndFile(t *testing.T) {
	f, err := os.CreateTemp("", "log")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	defer os.Remove(f.Name())

	l := NewLogger(&ConfigPushProxy{
		EnableConsoleLog: true,
		EnableFileLog:    true,
		LogFileLocation:  f.Name(),
	})

	// Resetting outputs to be consistent
	var infoBuf, errBuf bytes.Buffer
	l.cInfoLogger.SetOutput(&infoBuf)
	l.cErrLogger.SetOutput(&errBuf)
	l.cInfoLogger.SetFlags(0)
	l.cErrLogger.SetFlags(0)
	l.fInfoLogger.SetFlags(0)
	l.fErrLogger.SetFlags(0)

	t.Run("Info", func(t *testing.T) {
		l.Info("hello world")
	})

	t.Run("Infof", func(t *testing.T) {
		l.Infof("param %d", 1)
	})

	t.Run("Error", func(t *testing.T) {
		l.Error("hello error")
	})

	t.Run("Errorf", func(t *testing.T) {
		l.Errorf("err %d", 1)
	})

	buf, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	var total []byte
	total = append(total, infoBuf.Bytes()...)
	total = append(total, errBuf.Bytes()...)

	assert.True(t, bytes.Equal(buf, total))

	t.Run("Panic", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
		}()

		l.Panic("something")
	})

	t.Run("Panicf", func(t *testing.T) {
		defer func() {
			r := recover()
			require.NotNil(t, r)
		}()

		l.Panicf("param %d", 1)
	})
}

func TestLoggerConsole(t *testing.T) {
	f, err := os.CreateTemp("", "log")
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	defer os.Remove(f.Name())

	l := NewLogger(&ConfigPushProxy{
		EnableConsoleLog: true,
		EnableFileLog:    false,
		LogFileLocation:  f.Name(),
	})

	// Resetting outputs to be consistent
	var infoBuf, errBuf bytes.Buffer
	l.cInfoLogger.SetOutput(&infoBuf)
	l.cErrLogger.SetOutput(&errBuf)

	t.Run("Info", func(t *testing.T) {
		l.Info("hello world")
	})

	t.Run("Infof", func(t *testing.T) {
		l.Infof("param %d", 1)
	})

	t.Run("Error", func(t *testing.T) {
		l.Error("hello error")
	})

	t.Run("Errorf", func(t *testing.T) {
		l.Errorf("err %d", 1)
	})

	buf, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Empty(t, buf)
}
