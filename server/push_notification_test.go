// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
		want  string
	}{
		{"empty", "", ""},
		{"short token passes through", "1234", "1234"},
		{"16 chars passes through", "0123456789abcdef", "0123456789abcdef"},
		{"17 chars truncated", "0123456789abcdefg", "0123456789abcdef…"},
		{"long token truncated", "abcdef0123456789cafebabe1234", "abcdef0123456789…"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redactToken(tc.token))
		})
	}
}
