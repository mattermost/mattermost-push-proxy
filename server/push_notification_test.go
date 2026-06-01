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
		{"8 chars passes through", "12345678", "12345678"},
		{"9 chars truncated", "123456789", "12345678…"},
		{"long token truncated", "abcdef0123456789cafebabe", "abcdef01…"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RedactToken(tc.token))
		})
	}
}
