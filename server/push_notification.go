// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

// redactToken returns the first 8 chars of a device token followed by an
// ellipsis, for safe inclusion in logs.
func redactToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "…"
}
