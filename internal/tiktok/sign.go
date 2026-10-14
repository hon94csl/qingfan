package tiktok

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
)

// Sign is the HMAC-SHA256 request signature for API version 202309 and later.
// The access token is not part of the signed string. It travels in x-tts-access-token.
func Sign(secret, path string, query url.Values, body string) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		if key == "sign" || key == "access_token" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var signed string
	for _, key := range keys {
		signed += key + query.Get(key)
	}
	payload := secret + path + signed + body + secret
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
