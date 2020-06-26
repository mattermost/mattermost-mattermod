package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendToWebhook(t *testing.T) {
	validPayload := &Payload{Username: "mattermod", Text: "test"}

	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))
	defer mattermost.Close()

	r, err := s.sendToWebhook(mattermost.URL, validPayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	got := r.StatusCode
	if got != http.StatusOK {
		t.Errorf("got: %v, want: %v", got, http.StatusOK)
	}
	defer r.Body.Close()
}

func TestSendToWebhookUsernameNotSet(t *testing.T) {
	invalidPayload := &Payload{Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	wantError := "username not set in webhook payload"
	r, err := s.sendToWebhook(mattermost.URL, invalidPayload)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != wantError {
		t.Fatalf("got: %v, want: %v", err.Error(), wantError)
	}

	if r == nil {
		t.Fatal("expected response")
	}

	wantStatusCode := http.StatusBadRequest
	gotStatusCode := r.StatusCode
	if gotStatusCode != wantStatusCode {
		t.Errorf("got: %v, want: %v", gotStatusCode, wantStatusCode)
	}
	defer r.Body.Close()
}

func TestSendToWebhookWebhookURLNotSet(t *testing.T) {
	validPayload := &Payload{Username: "mattermod", Text: "test"}
	mattermost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer mattermost.Close()

	wantError := "no Mattermost webhook URL set: unable to send message"
	r, err := s.sendToWebhook("", validPayload)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != wantError {
		t.Fatalf("got: %v, want: %v", err.Error(), wantError)
	}

	if r == nil {
		t.Fatal("expected response")
	}

	wantStatusCode := http.StatusBadRequest
	gotStatusCode := r.StatusCode
	if gotStatusCode != wantStatusCode {
		t.Errorf("got: %v, want: %v", gotStatusCode, wantStatusCode)
	}
	defer r.Body.Close()
}
