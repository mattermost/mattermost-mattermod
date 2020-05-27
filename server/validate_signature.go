package server

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // GitHub webhooks are signed using sha1 https://developer.github.com/webhooks/.
	"encoding/hex"
	"errors"
	"fmt"
)

// ValidateSignature function used for validation of webhook requests based on
// config secret.
func ValidateSignature(receivedHash []string, bodyBuffer []byte, secretKey string) error {
	hash := hmac.New(sha1.New, []byte(secretKey))
	if _, err := hash.Write(bodyBuffer); err != nil {
		msg := fmt.Sprintf("Cannot compute the HMAC for request: %s\n", err)
		return errors.New(msg)
	}

	expectedHash := hex.EncodeToString(hash.Sum(nil))
	if receivedHash[1] != expectedHash {
		msg := fmt.Sprintf("Expected Hash does not match the received hash: %s\n", expectedHash)
		return errors.New(msg)
	}

	return nil
}
