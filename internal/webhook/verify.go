package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func verifySignature(secret string, body []byte, header string) error {
	if secret == "" {
		return fmt.Errorf("webhook secret not configured")
	}
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return fmt.Errorf("invalid signature header")
	}
	wantHex := strings.TrimPrefix(header, prefix)
	want, err := hex.DecodeString(wantHex)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := mac.Sum(nil)
	if !hmac.Equal(got, want) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
