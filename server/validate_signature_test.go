package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWithValidationMiddleware(t *testing.T) {
	s := &Server{
		StartTime: time.Now(),
		Config: &Config{
			Org:                 "mattertest",
			GitHubWebhookSecret: "3dca279e731c97c38e3019a075dee9ebbd0a99f1",
		},
	}

	handler := s.withValidation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	do := func(t *testing.T, signature, body string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(body))
		require.NoError(t, err)
		if signature != "" {
			req.Header.Set("X-Hub-Signature-256", signature)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(got)
	}

	secret := s.Config.GitHubWebhookSecret

	t.Run("valid signature passes the body on", func(t *testing.T) {
		status, body := do(t, sign(secret, []byte("some")), "some")
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, "some", body)
	})

	for name, tc := range map[string]struct {
		signature string
		body      string
		status    int
	}{
		"missing signature":                 {"", "some", http.StatusBadRequest},
		"wrong algorithm prefix":            {"sha1=3dca279e731c97c38e3019a075dee9ebbd0a99f0", "some", http.StatusBadRequest},
		"no value after prefix":             {"sha256=", "some", http.StatusUnauthorized},
		"signature for a different body":    {sign(secret, []byte("other")), "some", http.StatusUnauthorized},
		"signature with a different secret": {sign("wrong", []byte("some")), "some", http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			status, _ := do(t, tc.signature, tc.body)
			require.Equal(t, tc.status, status)
		})
	}

	t.Run("body too large", func(t *testing.T) {
		body := strings.Repeat("a", maxWebhookBodyBytes+1)
		status, _ := do(t, sign(secret, []byte(body)), body)
		require.Equal(t, http.StatusRequestEntityTooLarge, status)
	})

	t.Run("healthz skips validation", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/healthz")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestValidSignature(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	signature := strings.TrimPrefix(sign("secret", body), "sha256=")

	require.True(t, validSignature(signature, body, "secret"))
	require.False(t, validSignature(signature, body, ""))
	require.False(t, validSignature("not-hex", body, "secret"))
	require.False(t, validSignature(strings.ToUpper(signature)+"00", body, "secret"))
}
