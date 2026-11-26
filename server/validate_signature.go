package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// GitHub caps webhook payloads at 25 MB.
const maxWebhookBodyBytes = 25 << 20

func (s *Server) withValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		signature, ok := strings.CutPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		if !ok {
			mlog.Error("Missing or malformed webhook signature header")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		buf, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes))
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				mlog.Error("Webhook body too large")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			mlog.Error("Failed to read body", mlog.Err(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(buf))

		if !validSignature(signature, buf, s.Config.GitHubWebhookSecret) {
			mlog.Error("Webhook signature does not match")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validSignature reports whether signature is the hex HMAC-SHA256 of body
// under secret. An empty secret never validates.
func validSignature(signature string, body []byte, secret string) bool {
	if secret == "" {
		return false
	}

	received, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(received, mac.Sum(nil))
}
