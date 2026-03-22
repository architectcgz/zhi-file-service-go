package token

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/services/access/domain"
	"github.com/architectcgz/zhi-file-service-go/internal/services/access/ports"
)

const ticketPrefix = "at_"

type HMACTicketIssuer struct {
	key []byte
}

type signedClaims struct {
	FileID       string `json:"fileId"`
	TenantID     string `json:"tenantId"`
	Subject      string `json:"subject"`
	SubjectType  string `json:"subjectType,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	ResponseName string `json:"responseName,omitempty"`
	ExpiresAt    int64  `json:"expiresAt"`
}

func NewHMACTicketIssuer(signingKey string) (*HMACTicketIssuer, error) {
	signingKey = strings.TrimSpace(signingKey)
	if signingKey == "" {
		return nil, errors.New("ticket signing key is required")
	}
	return &HMACTicketIssuer{key: []byte(signingKey)}, nil
}

func (i *HMACTicketIssuer) Issue(_ context.Context, claims domain.AccessTicketClaims) (string, error) {
	if err := claims.Validate(); err != nil {
		return "", err
	}

	payload, err := json.Marshal(signedClaims{
		FileID:       claims.FileID,
		TenantID:     claims.TenantID,
		Subject:      claims.Subject,
		SubjectType:  claims.SubjectType,
		Disposition:  string(claims.Disposition),
		ResponseName: claims.ResponseName,
		ExpiresAt:    claims.ExpiresAt.UTC().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal ticket claims: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := i.sign(encodedPayload)
	return ticketPrefix + encodedPayload + "." + signature, nil
}

func (i *HMACTicketIssuer) Verify(_ context.Context, ticket string) (domain.AccessTicketClaims, error) {
	ticket = strings.TrimSpace(ticket)
	if !strings.HasPrefix(ticket, ticketPrefix) {
		return domain.AccessTicketClaims{}, errors.New("invalid ticket prefix")
	}

	body := strings.TrimPrefix(ticket, ticketPrefix)
	parts := strings.SplitN(body, ".", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return domain.AccessTicketClaims{}, errors.New("invalid ticket format")
	}

	expected := i.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return domain.AccessTicketClaims{}, errors.New("invalid ticket signature")
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return domain.AccessTicketClaims{}, fmt.Errorf("decode ticket payload: %w", err)
	}

	var payload signedClaims
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return domain.AccessTicketClaims{}, fmt.Errorf("unmarshal ticket payload: %w", err)
	}

	return domain.AccessTicketClaims{
		FileID:       payload.FileID,
		TenantID:     payload.TenantID,
		Subject:      payload.Subject,
		SubjectType:  payload.SubjectType,
		Disposition:  domain.DownloadDisposition(payload.Disposition),
		ResponseName: payload.ResponseName,
		ExpiresAt:    time.Unix(payload.ExpiresAt, 0).UTC(),
	}, nil
}

func (i *HMACTicketIssuer) sign(encodedPayload string) string {
	mac := hmac.New(sha256.New, i.key)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

var _ ports.AccessTicketIssuer = (*HMACTicketIssuer)(nil)
