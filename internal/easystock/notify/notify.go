// Package notify isolates SMS and push providers.
// A development provider records the message and must never be treated as a real delivery.
package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrAmbiguous means the provider may have accepted the message.
// Callers must not send it again automatically.
var ErrAmbiguous = errors.New("ambiguous provider result")

// ErrTransient is a retryable provider failure.
var ErrTransient = errors.New("transient provider failure")

// SMSProvider sends one SMS. idempotencyKey is forwarded when the provider supports it.
type SMSProvider interface {
	Send(ctx context.Context, to, body, idempotencyKey string) (providerID string, err error)
}

// PushProvider sends one mobile notification.
type PushProvider interface {
	Send(ctx context.Context, deviceToken, title, body, idempotencyKey string) (providerID string, err error)
}

// Message is a captured development delivery. It was not sent to a real handset.
type Message struct {
	To             string
	Title          string
	Body           string
	IdempotencyKey string
	Simulated      bool
}

// DevSMS records SMS messages. It refuses to run when Production is true.
type DevSMS struct {
	Production bool
	mu         sync.Mutex
	Messages   []Message
}

func (d *DevSMS) Send(_ context.Context, to, body, idempotencyKey string) (string, error) {
	if d.Production {
		return "", errors.New("le fournisseur SMS simulé est interdit en production")
	}
	d.mu.Lock()
	d.Messages = append(d.Messages, Message{To: to, Body: body, IdempotencyKey: idempotencyKey, Simulated: true})
	d.mu.Unlock()
	log.Printf("SMS simulé, non envoyé, destinataire %s", maskPhone(to))
	return "dev-simulated:" + idempotencyKey, nil
}

func (d *DevSMS) Bodies() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.Messages))
	for i := range d.Messages {
		out[i] = d.Messages[i].Body
	}
	return out
}

// DevPush records push messages and refuses to run in production.
type DevPush struct {
	Production bool
	mu         sync.Mutex
	Messages   []Message
}

func (d *DevPush) Send(_ context.Context, token, title, body, idempotencyKey string) (string, error) {
	if d.Production {
		return "", errors.New("le fournisseur push simulé est interdit en production")
	}
	d.mu.Lock()
	d.Messages = append(d.Messages, Message{To: token, Title: title, Body: body, IdempotencyKey: idempotencyKey, Simulated: true})
	d.mu.Unlock()
	log.Printf("notification push simulée, non envoyée")
	return "dev-simulated:" + idempotencyKey, nil
}

func (d *DevPush) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.Messages)
}

// HTTPSMS posts JSON to a configured gateway. A timeout is ambiguous: the gateway may have accepted the SMS.
type HTTPSMS struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func (h *HTTPSMS) Send(ctx context.Context, to, body, idempotencyKey string) (string, error) {
	if h.URL == "" {
		return "", errors.New("SMS_HTTP_URL est vide")
	}
	client := h.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	payload := fmt.Sprintf(`{"to":%q,"body":%q,"idempotency_key":%q}`, to, body, idempotencyKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	req.Header.Set("Idempotency-Key", idempotencyKey)
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return "", ErrAmbiguous
		}
		return "", fmt.Errorf("%w: %s", ErrTransient, "passerelle SMS injoignable")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 500 {
		return "", ErrTransient
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("passerelle SMS a refusé l'envoi (%d)", resp.StatusCode)
	}
	id := strings.TrimSpace(string(raw))
	if id == "" {
		id = idempotencyKey
	}
	return id, nil
}

// HTTPPush posts JSON to a configured push gateway.
type HTTPPush struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func (h *HTTPPush) Send(ctx context.Context, deviceToken, title, body, idempotencyKey string) (string, error) {
	if h.URL == "" {
		return "", errors.New("PUSH_HTTP_URL est vide")
	}
	client := h.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	payload := fmt.Sprintf(`{"token":%q,"title":%q,"body":%q,"idempotency_key":%q}`, deviceToken, title, body, idempotencyKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	req.Header.Set("Idempotency-Key", idempotencyKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", ErrTransient
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", ErrTransient
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("passerelle push a refusé l'envoi (%d)", resp.StatusCode)
	}
	return idempotencyKey, nil
}

func maskPhone(phone string) string {
	if len(phone) < 6 {
		return "***"
	}
	return phone[:4] + "****" + phone[len(phone)-2:]
}

// OpenSMS returns the configured provider.
// provider "dev" is refused when production is true.
func OpenSMS(provider, url, token string, production bool) (SMSProvider, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "dev":
		if production {
			return nil, errors.New("SMS_PROVIDER=dev est interdit lorsque APP_ENV=production")
		}
		return &DevSMS{}, nil
	case "http":
		if url == "" {
			return nil, errors.New("SMS_HTTP_URL est requis lorsque SMS_PROVIDER=http")
		}
		return &HTTPSMS{URL: url, Token: token}, nil
	default:
		return nil, fmt.Errorf("SMS_PROVIDER inconnu: %s", provider)
	}
}

// OpenPush returns the configured push provider.
func OpenPush(provider, url, token string, production bool) (PushProvider, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "dev":
		if production {
			return nil, errors.New("PUSH_PROVIDER=dev est interdit lorsque APP_ENV=production")
		}
		return &DevPush{}, nil
	case "http":
		if url == "" {
			return nil, errors.New("PUSH_HTTP_URL est requis lorsque PUSH_PROVIDER=http")
		}
		return &HTTPPush{URL: url, Token: token}, nil
	default:
		return nil, fmt.Errorf("PUSH_PROVIDER inconnu: %s", provider)
	}
}
