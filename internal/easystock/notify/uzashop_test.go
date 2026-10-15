package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUzaSMSSend(t *testing.T) {
	var got uzaSendRequest
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/sms/send" || r.Method != http.MethodPost {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"SMS sent successfully","data":{"messageId":2026101514042604900,"status":"SUBMITTED","phoneNumber":"243970000001","remainingBalance":"360.00"}}`))
	}))
	defer srv.Close()

	provider := &UzaSMS{BaseURL: srv.URL, Token: "secret-token", SenderName: "EasyStock", HTTP: srv.Client()}
	id, err := provider.Send(context.Background(), "+243970000001", "EasyStock: code 123456. Valable 5 min.", "otp:abc")
	if err != nil {
		t.Fatal(err)
	}
	if id != "2026101514042604900" {
		t.Fatalf("id %s", id)
	}
	if got.TokenKey != "secret-token" || got.PhoneNumber != "243970000001" || got.SenderName != "EasyStock" {
		t.Fatalf("payload %+v", got)
	}
	if got.Message != "EasyStock: code 123456. Valable 5 min." {
		t.Fatalf("message %q", got.Message)
	}
	if auth != "" {
		t.Fatalf("authorization header %q", auth)
	}
}

func TestUzaSMSRejects(t *testing.T) {
	provider := &UzaSMS{BaseURL: "https://sms.uzashop.co", Token: "tok", SenderName: "EasyStock"}
	if _, err := provider.Send(context.Background(), "+243970000001", strings.Repeat("a", 161), "k"); err == nil {
		t.Fatal("long message was accepted")
	}
	if _, err := provider.Send(context.Background(), "24 39", "ok", "k"); err == nil {
		t.Fatal("spaced number was accepted")
	}

	cases := []struct {
		status    int
		body      string
		transient bool
		ambiguous bool
	}{
		{http.StatusPaymentRequired, `{"success":false,"message":"Insufficient SMS balance"}`, false, false},
		{http.StatusNotFound, `{"success":false,"message":"Subscription not found or inactive"}`, false, false},
		{http.StatusBadRequest, `{"success":false,"message":"Validation failed"}`, false, false},
		{http.StatusInternalServerError, `{"success":false}`, true, false},
		{http.StatusTooManyRequests, `{}`, true, false},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		p := &UzaSMS{BaseURL: srv.URL, Token: "tok", SenderName: "EasyStock", HTTP: srv.Client()}
		_, err := p.Send(context.Background(), "+243970000001", "bonjour", "k")
		srv.Close()
		if err == nil {
			t.Fatalf("status %d returned nil", tc.status)
		}
		if errors.Is(err, ErrTransient) != tc.transient {
			t.Fatalf("status %d transient=%v err=%v", tc.status, tc.transient, err)
		}
		if errors.Is(err, ErrAmbiguous) != tc.ambiguous {
			t.Fatalf("status %d ambiguous=%v err=%v", tc.status, tc.ambiguous, err)
		}
		if strings.Contains(err.Error(), "tok") || strings.Contains(err.Error(), "bonjour") {
			t.Fatalf("error leaked payload: %v", err)
		}
	}
}

func TestUzaSMSTimeoutIsAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	provider := &UzaSMS{
		BaseURL:    srv.URL,
		Token:      "tok",
		SenderName: "EasyStock",
		HTTP:       &http.Client{Timeout: 30 * time.Millisecond},
	}
	_, err := provider.Send(context.Background(), "+243970000001", "bonjour", "k")
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err %v", err)
	}
}

func TestOpenSMSUzaShop(t *testing.T) {
	provider, err := OpenSMS(SMSOptions{Provider: "uzashop", UzaToken: "abc", Production: true})
	if err != nil {
		t.Fatal(err)
	}
	uza, ok := provider.(*UzaSMS)
	if !ok {
		t.Fatalf("type %T", provider)
	}
	if uza.BaseURL != uzaDefaultBaseURL || uza.SenderName != uzaDefaultSender || uza.Token != "abc" {
		t.Fatalf("provider %+v", uza)
	}
	if _, err := OpenSMS(SMSOptions{Provider: "uzashop", Production: true}); err == nil {
		t.Fatal("missing token was accepted")
	}
	if _, err := OpenSMS(SMSOptions{Provider: "uzashop", UzaToken: "abc", UzaSender: "NOM_TROP_LONG"}); err == nil {
		t.Fatal("long sender was accepted")
	}
	if _, err := OpenSMS(SMSOptions{Provider: "dev", Production: true}); err == nil {
		t.Fatal("dev provider was accepted in production")
	}
}
