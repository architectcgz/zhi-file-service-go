package dataplane

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const (
	RequiredAudience = "zhi-file-data-plane"

	jwksFetchTimeout  = 5 * time.Second
	jwksResponseLimit = 1 << 20
	maxIATFutureSkew  = 60 * time.Second
)

var errUnsupportedJWKSKey = errors.New("unsupported jwks signing key")

type StandardClaims struct {
	SubjectID   string
	SubjectType string
	TenantID    string
	ClientID    string
	TokenID     string
	Scopes      []string

	Issuer    string
	Audience  []string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type Resolver func(*http.Request) (StandardClaims, error)

type jwksResolver struct {
	keySet         *jwksKeySet
	allowedIssuers map[string]struct{}
	now            func() time.Time
}

type jwksKeySet struct {
	url    string
	client *http.Client

	mu   sync.RWMutex
	keys []jwkKey
}

type jwkKey struct {
	id        string
	algorithm string
	publicKey crypto.PublicKey
}

type jwksDocument struct {
	Keys []jwkDocument `json:"keys"`
}

type jwkDocument struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
}

type jwtHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type jwtClaims map[string]json.RawMessage

func NewJWKSResolver(source string) (Resolver, error) {
	return NewJWKSResolverWithIssuers(source, nil)
}

func NewJWKSResolverWithIssuers(source string, allowedIssuers []string) (Resolver, error) {
	keySet, err := newJWKSKeySet(source)
	if err != nil {
		return nil, fmt.Errorf("new jwks key set: %w", err)
	}

	resolver := jwksResolver{
		keySet:         keySet,
		allowedIssuers: normalizeStringSet(allowedIssuers),
		now:            time.Now,
	}
	return resolver.resolve, nil
}

func (r jwksResolver) resolve(req *http.Request) (StandardClaims, error) {
	token, ok := bearerTokenFromRequest(req)
	if !ok {
		return StandardClaims{}, newUnauthorizedAuthError()
	}

	header, claims, signingInput, signature, err := parseJWT(token)
	if err != nil {
		return StandardClaims{}, newUnauthorizedAuthError()
	}

	key, err := r.keySet.key(header.KeyID)
	if err != nil {
		return StandardClaims{}, newUnauthorizedAuthError()
	}
	if err := verifyJWTWithKey(header, key, signingInput, signature); err != nil {
		if !r.keySet.isRemote() {
			return StandardClaims{}, newUnauthorizedAuthError()
		}
		key, retryErr := r.keySet.refreshKey(header.KeyID)
		if retryErr != nil || verifyJWTWithKey(header, key, signingInput, signature) != nil {
			return StandardClaims{}, newUnauthorizedAuthError()
		}
	}

	issuer, err := requiredStringClaim(claims, "iss")
	if err != nil {
		return StandardClaims{}, newUnauthorizedAuthError()
	}
	if len(r.allowedIssuers) > 0 {
		if _, ok := r.allowedIssuers[issuer]; !ok {
			return StandardClaims{}, newUnauthorizedAuthError()
		}
	}

	standardClaims, err := standardClaimsFromJWT(claims, r.now())
	if err != nil {
		return StandardClaims{}, newUnauthorizedAuthError()
	}
	return standardClaims, nil
}

func newJWKSKeySet(source string) (*jwksKeySet, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, fmt.Errorf("jwks source is required")
	}

	if strings.HasPrefix(trimmed, "{") {
		keys, err := parseJWKS([]byte(trimmed))
		if err != nil {
			return nil, err
		}
		return &jwksKeySet{keys: keys}, nil
	}

	parsedURL, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse jwks url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("jwks url scheme %q is unsupported", parsedURL.Scheme)
	}

	keySet := &jwksKeySet{
		url:    parsedURL.String(),
		client: &http.Client{Timeout: jwksFetchTimeout},
	}
	if err := keySet.refresh(); err != nil {
		return nil, err
	}
	return keySet, nil
}

func (s *jwksKeySet) key(keyID string) (jwkKey, error) {
	if key, ok := s.cachedKey(keyID); ok {
		return key, nil
	}
	if s.url == "" {
		return jwkKey{}, fmt.Errorf("jwks key %q not found", keyID)
	}
	if err := s.refresh(); err != nil {
		return jwkKey{}, err
	}
	if key, ok := s.cachedKey(keyID); ok {
		return key, nil
	}
	return jwkKey{}, fmt.Errorf("jwks key %q not found", keyID)
}

func (s *jwksKeySet) refreshKey(keyID string) (jwkKey, error) {
	if !s.isRemote() {
		return jwkKey{}, fmt.Errorf("jwks source is not remote")
	}
	if err := s.refresh(); err != nil {
		return jwkKey{}, err
	}
	if key, ok := s.cachedKey(keyID); ok {
		return key, nil
	}
	return jwkKey{}, fmt.Errorf("jwks key %q not found", keyID)
}

func (s *jwksKeySet) isRemote() bool {
	return s.url != ""
}

func (s *jwksKeySet) cachedKey(keyID string) (jwkKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if keyID == "" && len(s.keys) == 1 {
		return s.keys[0], true
	}
	for _, key := range s.keys {
		if key.id == keyID {
			return key, true
		}
	}
	return jwkKey{}, false
}

func (s *jwksKeySet) refresh() error {
	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("create jwks request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, jwksResponseLimit))
	if err != nil {
		return fmt.Errorf("read jwks: %w", err)
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.keys = keys
	s.mu.Unlock()
	return nil
}

func parseJWKS(body []byte) ([]jwkKey, error) {
	var document jwksDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("parse jwks json: %w", err)
	}
	if len(document.Keys) == 0 {
		return nil, fmt.Errorf("jwks must contain at least one key")
	}

	keys := make([]jwkKey, 0, len(document.Keys))
	for _, documentKey := range document.Keys {
		if use := strings.TrimSpace(documentKey.Use); use != "" && use != "sig" {
			continue
		}

		publicKey, err := parseJWKSKey(documentKey)
		if err != nil {
			if errors.Is(err, errUnsupportedJWKSKey) {
				continue
			}
			return nil, err
		}
		keys = append(keys, jwkKey{
			id:        strings.TrimSpace(documentKey.KeyID),
			algorithm: strings.TrimSpace(documentKey.Algorithm),
			publicKey: publicKey,
		})
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks does not contain usable signing keys")
	}
	return keys, nil
}

func parseJWKSKey(documentKey jwkDocument) (crypto.PublicKey, error) {
	switch strings.TrimSpace(documentKey.KeyType) {
	case "RSA":
		return parseRSAJWKSKey(documentKey)
	case "EC":
		return parseECJWKSKey(documentKey)
	default:
		return nil, fmt.Errorf("%w: jwks key type %q", errUnsupportedJWKSKey, documentKey.KeyType)
	}
}

func parseRSAJWKSKey(documentKey jwkDocument) (*rsa.PublicKey, error) {
	modulus, err := decodeBase64URLInt(documentKey.Modulus)
	if err != nil {
		return nil, fmt.Errorf("decode jwks rsa modulus: %w", err)
	}
	exponent, err := decodeBase64URLInt(documentKey.Exponent)
	if err != nil {
		return nil, fmt.Errorf("decode jwks rsa exponent: %w", err)
	}
	if exponent.Sign() <= 0 || !exponent.IsInt64() {
		return nil, fmt.Errorf("jwks rsa exponent is invalid")
	}

	return &rsa.PublicKey{
		N: modulus,
		E: int(exponent.Int64()),
	}, nil
}

func parseECJWKSKey(documentKey jwkDocument) (*ecdsa.PublicKey, error) {
	curve, err := curveByName(documentKey.Curve)
	if err != nil {
		return nil, err
	}
	x, err := decodeBase64URLInt(documentKey.X)
	if err != nil {
		return nil, fmt.Errorf("decode jwks ec x: %w", err)
	}
	y, err := decodeBase64URLInt(documentKey.Y)
	if err != nil {
		return nil, fmt.Errorf("decode jwks ec y: %w", err)
	}
	if !curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("jwks ec point is invalid")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func curveByName(name string) (elliptic.Curve, error) {
	switch strings.TrimSpace(name) {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("%w: jwks ec curve %q", errUnsupportedJWKSKey, name)
	}
}

func parseJWT(token string) (jwtHeader, jwtClaims, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("jwt must contain three parts")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("decode jwt header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("parse jwt header: %w", err)
	}
	header.Algorithm = strings.TrimSpace(header.Algorithm)
	if header.Algorithm == "" || strings.EqualFold(header.Algorithm, "none") {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("jwt alg is invalid")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("decode jwt claims: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("parse jwt claims: %w", err)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, nil, "", nil, fmt.Errorf("decode jwt signature: %w", err)
	}
	return header, claims, parts[0] + "." + parts[1], signature, nil
}

func verifyJWTSignature(algorithm string, publicKey crypto.PublicKey, signingInput string, signature []byte) error {
	switch strings.TrimSpace(algorithm) {
	case "RS256":
		return verifyRSASignature(crypto.SHA256, publicKey, signingInput, signature)
	case "RS384":
		return verifyRSASignature(crypto.SHA384, publicKey, signingInput, signature)
	case "RS512":
		return verifyRSASignature(crypto.SHA512, publicKey, signingInput, signature)
	case "PS256":
		return verifyRSAPSSSignature(crypto.SHA256, publicKey, signingInput, signature)
	case "PS384":
		return verifyRSAPSSSignature(crypto.SHA384, publicKey, signingInput, signature)
	case "PS512":
		return verifyRSAPSSSignature(crypto.SHA512, publicKey, signingInput, signature)
	case "ES256":
		return verifyECDSASignature(crypto.SHA256, publicKey, signingInput, signature)
	case "ES384":
		return verifyECDSASignature(crypto.SHA384, publicKey, signingInput, signature)
	case "ES512":
		return verifyECDSASignature(crypto.SHA512, publicKey, signingInput, signature)
	default:
		return fmt.Errorf("jwt alg %q is unsupported", algorithm)
	}
}

func verifyJWTWithKey(header jwtHeader, key jwkKey, signingInput string, signature []byte) error {
	if key.algorithm != "" && !strings.EqualFold(key.algorithm, header.Algorithm) {
		return fmt.Errorf("jwt algorithm does not match jwks key")
	}
	return verifyJWTSignature(header.Algorithm, key.publicKey, signingInput, signature)
}

func verifyRSASignature(hash crypto.Hash, publicKey crypto.PublicKey, signingInput string, signature []byte) error {
	key, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("jwt key type does not match rsa algorithm")
	}
	return rsa.VerifyPKCS1v15(key, hash, hashJWTInput(hash, signingInput), signature)
}

func verifyRSAPSSSignature(hash crypto.Hash, publicKey crypto.PublicKey, signingInput string, signature []byte) error {
	key, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("jwt key type does not match rsa algorithm")
	}
	return rsa.VerifyPSS(key, hash, hashJWTInput(hash, signingInput), signature, nil)
}

func verifyECDSASignature(hash crypto.Hash, publicKey crypto.PublicKey, signingInput string, signature []byte) error {
	key, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("jwt key type does not match ecdsa algorithm")
	}

	size := (key.Curve.Params().BitSize + 7) / 8
	if len(signature) != size*2 {
		return fmt.Errorf("jwt ecdsa signature length is invalid")
	}

	r := new(big.Int).SetBytes(signature[:size])
	s := new(big.Int).SetBytes(signature[size:])
	if !ecdsa.Verify(key, hashJWTInput(hash, signingInput), r, s) {
		return fmt.Errorf("jwt signature verification failed")
	}
	return nil
}

func hashJWTInput(hash crypto.Hash, input string) []byte {
	switch hash {
	case crypto.SHA256:
		sum := sha256.Sum256([]byte(input))
		return sum[:]
	case crypto.SHA384:
		sum := sha512.Sum384([]byte(input))
		return sum[:]
	case crypto.SHA512:
		sum := sha512.Sum512([]byte(input))
		return sum[:]
	default:
		return nil
	}
}

func standardClaimsFromJWT(claims jwtClaims, now time.Time) (StandardClaims, error) {
	subjectID, err := requiredStringClaim(claims, "sub")
	if err != nil {
		return StandardClaims{}, err
	}
	subjectType, err := requiredSubjectTypeClaim(claims, "subject_type")
	if err != nil {
		return StandardClaims{}, err
	}
	tenantID, err := requiredStringClaim(claims, "tenant_id")
	if err != nil {
		return StandardClaims{}, err
	}
	issuer, err := requiredStringClaim(claims, "iss")
	if err != nil {
		return StandardClaims{}, err
	}
	audiences, err := audienceClaim(claims, "aud")
	if err != nil {
		return StandardClaims{}, err
	}
	if !containsString(audiences, RequiredAudience) {
		return StandardClaims{}, fmt.Errorf("jwt audience does not contain %q", RequiredAudience)
	}
	issuedAt, err := requiredUnixTimeClaim(claims, "iat")
	if err != nil {
		return StandardClaims{}, err
	}
	if issuedAt.After(now.Add(maxIATFutureSkew)) {
		return StandardClaims{}, fmt.Errorf("jwt iat is in the future")
	}
	expiresAt, err := requiredUnixTimeClaim(claims, "exp")
	if err != nil {
		return StandardClaims{}, err
	}
	if !expiresAt.After(issuedAt) {
		return StandardClaims{}, fmt.Errorf("jwt exp must be after iat")
	}
	if !now.Before(expiresAt) {
		return StandardClaims{}, fmt.Errorf("jwt is expired")
	}
	if notBefore, ok, err := optionalUnixTimeClaim(claims, "nbf"); err != nil {
		return StandardClaims{}, err
	} else if ok && now.Before(notBefore) {
		return StandardClaims{}, fmt.Errorf("jwt is not active yet")
	}

	scopes, err := scopeClaim(claims, "scope")
	if err != nil {
		return StandardClaims{}, err
	}
	clientID, err := optionalStringClaim(claims, "client_id")
	if err != nil {
		return StandardClaims{}, err
	}
	tokenID, err := optionalStringClaim(claims, "jti")
	if err != nil {
		return StandardClaims{}, err
	}

	return StandardClaims{
		SubjectID:   subjectID,
		SubjectType: subjectType,
		TenantID:    tenantID,
		ClientID:    clientID,
		TokenID:     tokenID,
		Scopes:      scopes,
		Issuer:      issuer,
		Audience:    audiences,
		IssuedAt:    issuedAt,
		ExpiresAt:   expiresAt,
	}, nil
}

func requiredStringClaim(claims jwtClaims, key string) (string, error) {
	value, ok := claims[key]
	if !ok {
		return "", fmt.Errorf("jwt claim %q is required", key)
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", fmt.Errorf("jwt claim %q must be a string", key)
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf("jwt claim %q must not be empty", key)
	}
	return result, nil
}

func optionalStringClaim(claims jwtClaims, key string) (string, error) {
	value, ok := claims[key]
	if !ok {
		return "", nil
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", fmt.Errorf("jwt claim %q must be a string", key)
	}
	return strings.TrimSpace(result), nil
}

func requiredSubjectTypeClaim(claims jwtClaims, key string) (string, error) {
	value, err := requiredStringClaim(claims, key)
	if err != nil {
		return "", err
	}
	switch normalized := strings.ToUpper(value); normalized {
	case "USER", "APP":
		return normalized, nil
	default:
		return "", fmt.Errorf("jwt claim %q must be USER or APP", key)
	}
}

func audienceClaim(claims jwtClaims, key string) ([]string, error) {
	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("jwt claim %q is required", key)
	}

	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil, fmt.Errorf("jwt claim %q must not be empty", key)
		}
		return []string{single}, nil
	}

	var multiple []string
	if err := json.Unmarshal(value, &multiple); err != nil {
		return nil, fmt.Errorf("jwt claim %q must be a string or string array", key)
	}
	if len(multiple) == 0 {
		return nil, fmt.Errorf("jwt claim %q must not be empty", key)
	}

	result := make([]string, 0, len(multiple))
	for _, item := range multiple {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil, fmt.Errorf("jwt claim %q must not contain empty audience", key)
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func scopeClaim(claims jwtClaims, key string) ([]string, error) {
	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("jwt claim %q is required", key)
	}

	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		return parseScopeString(single, key)
	}

	var multiple []string
	if err := json.Unmarshal(value, &multiple); err == nil {
		return normalizeNonEmptyStringSlice(multiple, key)
	}

	return nil, fmt.Errorf("jwt claim %q must be a string or string array", key)
}

func parseScopeString(scopeValue string, key string) ([]string, error) {
	scopeValue = strings.TrimSpace(scopeValue)
	if scopeValue == "" {
		return nil, fmt.Errorf("jwt claim %q must not be empty", key)
	}
	parts := strings.Fields(scopeValue)
	if len(parts) == 0 {
		return nil, fmt.Errorf("jwt claim %q must not be empty", key)
	}
	return normalizeNonEmptyStringSlice(parts, key)
}

func normalizeNonEmptyStringSlice(values []string, key string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, fmt.Errorf("jwt claim %q must not contain empty entries", key)
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("jwt claim %q must not be empty", key)
	}
	return result, nil
}

func requiredUnixTimeClaim(claims jwtClaims, key string) (time.Time, error) {
	value, ok := claims[key]
	if !ok {
		return time.Time{}, fmt.Errorf("jwt claim %q is required", key)
	}

	var number json.Number
	if err := json.Unmarshal(value, &number); err == nil {
		timestamp, err := number.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("jwt claim %q must be an integer unix timestamp", key)
		}
		return time.Unix(timestamp, 0), nil
	}

	return time.Time{}, fmt.Errorf("jwt claim %q must be a unix timestamp", key)
}

func optionalUnixTimeClaim(claims jwtClaims, key string) (time.Time, bool, error) {
	value, ok := claims[key]
	if !ok {
		return time.Time{}, false, nil
	}

	var number json.Number
	if err := json.Unmarshal(value, &number); err != nil {
		return time.Time{}, false, fmt.Errorf("jwt claim %q must be a unix timestamp", key)
	}
	timestamp, err := number.Int64()
	if err != nil {
		return time.Time{}, false, fmt.Errorf("jwt claim %q must be an integer unix timestamp", key)
	}
	return time.Unix(timestamp, 0), true, nil
}

func decodeBase64URLInt(value string) (*big.Int, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("decoded value is empty")
	}
	return new(big.Int).SetBytes(decoded), nil
}

func bearerTokenFromRequest(req *http.Request) (string, bool) {
	if req == nil {
		return "", false
	}
	authorization := strings.TrimSpace(req.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(authorization[7:])
	if token == "" {
		return "", false
	}
	return token, true
}

func newUnauthorizedAuthError() error {
	return xerrors.New(xerrors.CodeUnauthorized, "bearer token is missing or invalid", nil)
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func normalizeStringSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
