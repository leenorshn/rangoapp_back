package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	uzaDefaultBaseURL  = "https://sms.uzashop.co"
	uzaSendPath        = "/api/v2/sms/send"
	uzaDefaultSender   = "EasyStock"
	uzaMaxSenderRunes  = 11
	uzaMaxMessageRunes = 160
)

// UzaSMS sends one SMS through UZASHOP SMS Management V2.
// POST {base}/api/v2/sms/send with tokenKey, phoneNumber, senderName and message.
// A timeout is ambiguous: the gateway may already have accepted the SMS.
type UzaSMS struct {
	BaseURL    string
	Token      string
	SenderName string
	HTTP       *http.Client
}

type uzaSendRequest struct {
	TokenKey    string `json:"tokenKey"`
	PhoneNumber string `json:"phoneNumber"`
	SenderName  string `json:"senderName"`
	Message     string `json:"message"`
}

type uzaSendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		MessageID uzaMessageID `json:"messageId"`
		Status    string       `json:"status"`
	} `json:"data"`
}

// uzaMessageID accepts the identifier as a JSON string or number.
// SMS Management V2 documents a string and returns a number.
type uzaMessageID string

func (id *uzaMessageID) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*id = uzaMessageID(text)
		return nil
	}
	*id = uzaMessageID(raw)
	return nil
}

func openUzaSMS(opts SMSOptions) (*UzaSMS, error) {
	token := strings.TrimSpace(opts.UzaToken)
	if token == "" {
		return nil, errors.New("SMS_UZA_TOKEN est requis lorsque SMS_PROVIDER=uzashop")
	}
	sender := strings.TrimSpace(opts.UzaSender)
	if sender == "" {
		sender = uzaDefaultSender
	}
	if utf8.RuneCountInString(sender) > uzaMaxSenderRunes {
		return nil, errors.New("SMS_UZA_SENDER dépasse 11 caractères")
	}
	base := strings.TrimSpace(opts.UzaBaseURL)
	if base == "" {
		base = uzaDefaultBaseURL
	}
	return &UzaSMS{BaseURL: base, Token: token, SenderName: sender}, nil
}

func (u *UzaSMS) Send(ctx context.Context, to, body, idempotencyKey string) (string, error) {
	if strings.TrimSpace(u.Token) == "" {
		return "", errors.New("SMS_UZA_TOKEN est vide")
	}
	phone, err := uzaPhone(to)
	if err != nil {
		return "", err
	}
	message := strings.TrimSpace(body)
	if message == "" {
		return "", errors.New("le SMS est vide")
	}
	if utf8.RuneCountInString(message) > uzaMaxMessageRunes {
		return "", errors.New("le SMS dépasse 160 caractères")
	}
	sender := strings.TrimSpace(u.SenderName)
	if sender == "" {
		sender = uzaDefaultSender
	}
	payload, err := json.Marshal(uzaSendRequest{
		TokenKey:    u.Token,
		PhoneNumber: phone,
		SenderName:  sender,
		Message:     message,
	})
	if err != nil {
		return "", err
	}
	endpoint, err := uzaEndpoint(u.BaseURL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := u.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", classifyUzaTransport(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", ErrAmbiguous
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return "", ErrTransient
	}
	var parsed uzaSendResponse
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil && resp.StatusCode < 300 {
			return "", ErrAmbiguous
		}
	}
	if resp.StatusCode >= 300 || !parsed.Success {
		return "", uzaRejected(resp.StatusCode)
	}
	status := strings.ToUpper(strings.TrimSpace(parsed.Data.Status))
	if status == "FAILED" || status == "REJECTED" {
		return "", fmt.Errorf("UZASHOP a refusé le SMS (%s)", status)
	}
	id := strings.TrimSpace(string(parsed.Data.MessageID))
	if id == "" {
		id = idempotencyKey
	}
	log.Printf("SMS UZASHOP accepté, destinataire %s, id %s", maskPhone(to), id)
	return id, nil
}

func uzaEndpoint(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = uzaDefaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("SMS_UZA_BASE_URL est invalide")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("SMS_UZA_BASE_URL est invalide")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, uzaSendPath) {
		path += uzaSendPath
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func uzaPhone(raw string) (string, error) {
	number := strings.TrimSpace(raw)
	number = strings.TrimPrefix(number, "+")
	if number == "" {
		return "", errors.New("numéro SMS vide")
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return "", errors.New("numéro SMS invalide")
		}
	}
	return number, nil
}

func classifyUzaTransport(err error) error {
	var urlErr *url.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &urlErr) && urlErr.Timeout()) {
		return ErrAmbiguous
	}
	return fmt.Errorf("%w: passerelle SMS injoignable", ErrTransient)
}

func uzaRejected(status int) error {
	switch status {
	case http.StatusPaymentRequired:
		return errors.New("solde SMS UZASHOP insuffisant")
	case http.StatusNotFound:
		return errors.New("abonnement SMS UZASHOP introuvable")
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return errors.New("UZASHOP a refusé le SMS")
	default:
		return fmt.Errorf("UZASHOP a refusé le SMS (%d)", status)
	}
}
