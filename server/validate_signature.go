package server

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
)

func validateSignature(recievedHash []string, bodyBuffer []byte, key string) error {
	hash := hmac.New(sha1.New, []byte(key))
	if _, err := hash.Write(bodyBuffer); err != nil {
		msg := fmt.Sprintf("Cannot compute the HMAC for request: %s\n", err)
		return errors.New(msg)
	}

	expectedHash := hex.EncodeToString(hash.Sum(nil))
	if recievedHash[1] != expectedHash {
		msg := fmt.Sprintf("Expected Hash does not match the recieved hash: %s\n", expectedHash)
		return errors.New(msg)
	}

	return nil
}
