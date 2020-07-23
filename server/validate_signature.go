package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // GitHub webhooks are signed using sha1 https://developer.github.com/webhooks/.
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/mlog"
)

func (s *Server) withValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/healthz" {
			next.ServeHTTP(w, r)
		}
		receivedHash := strings.SplitN(r.Header.Get("X-Hub-Signature"), "=", 2)
		if receivedHash[0] != "sha1" {
			mlog.Error("Invalid webhook hash signature: SHA1")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			mlog.Error("Failed to read body", mlog.Err(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

		err = validateSignature(receivedHash, buf, s.Config.GitHubWebhookSecret)
		if err != nil {
			mlog.Error(err.Error())
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateSignature function used for validation of webhook requests based on
// config secret.
func validateSignature(receivedHash []string, bodyBuffer []byte, secretKey string) error {
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
