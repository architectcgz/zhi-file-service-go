package httptransport

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

	"github.com/architectcgz/zhi-file-service-go/internal/services/admin/domain"
	"github.com/architectcgz/zhi-file-service-go/pkg/xerrors"
)

const (
	adminJWKSRequiredAudience = "zhi-file-admin"
	jwksFetchTimeout          = 5 * time.Second
	jwksResponseLimit         = 1 << 20
)

var errUnsupportedJWKSKey = errors.New("unsupported jwks signing key")

type jwksAuthResolver struct {
	keySet *jwksKeySet
	now    func() time.Time
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

func NewJWKSAuthResolver(source string) (AuthFunc, error) {
	keySet, err := newJWKSKeySet(source)
	if err != nil {
		return nil, fmt.Errorf("new jwks key set: %w", err)
	}

	resolver := jwksAuthResolver{
		keySet: keySet,
		now:    time.Now,
	}
	return resolver.resolve, nil
}

func (r jwksAuthResolver) resolve(req *http.Request) (domain.AdminContext, error) {
	token, ok := bearerTokenFromRequest(req)
	if !ok {
		return domain.AdminContext{}, newUnauthorizedAuthError()
	}

	header, claims, signingInput, signature, err := parseJWT(token)
	if err != nil {
		return domain.AdminContext{}, newUnauthorizedAuthError()
	}

	key, err := r.keySet.key(header.KeyID)
	if err != nil {
		return domain.AdminContext{}, newUnauthorizedAuthError()
	}
	if err := verifyJWTWithKey(header, key, signingInput, signature); err != nil {
		if !r.keySet.isRemote() {
			return domain.AdminContext{}, newUnauthorizedAuthError()
		}
		key, retryErr := r.keySet.refreshKey(header.KeyID)
		if retryErr != nil || verifyJWTWithKey(header, key, signingInput, signature) != nil {
			return domain.AdminContext{}, newUnauthorizedAuthError()
		}
	}

	input, err := adminContextInputFromClaims(claims, requestIDFromRequest(req, ""), r.now())
	if err != nil {
		return domain.AdminContext{}, newUnauthorizedAuthError()
	}

	auth, err := domain.NewAdminContext(input)
	if err != nil {
		return domain.AdminContext{}, newUnauthorizedAuthError()
	}
	return auth, nil
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
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
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

func adminContextInputFromClaims(claims jwtClaims, requestID string, now time.Time) (domain.AdminContextInput, error) {
	adminID, err := requiredStringClaim(claims, "sub")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	if _, err := requiredStringClaim(claims, "iss"); err != nil {
		return domain.AdminContextInput{}, err
	}

	audiences, err := audienceClaim(claims, "aud")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	if !containsString(audiences, adminJWKSRequiredAudience) {
		return domain.AdminContextInput{}, fmt.Errorf("jwt audience does not contain %q", adminJWKSRequiredAudience)
	}

	if _, err := requiredUnixTimeClaim(claims, "iat"); err != nil {
		return domain.AdminContextInput{}, err
	}
	expiry, err := requiredUnixTimeClaim(claims, "exp")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	if !now.Before(expiry) {
		return domain.AdminContextInput{}, fmt.Errorf("jwt is expired")
	}

	roles, err := requiredStringSliceClaim(claims, "roles")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	tenantScopes, err := optionalStringSliceClaim(claims, "tenant_scopes")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	permissions, err := optionalStringSliceClaim(claims, "permissions")
	if err != nil {
		return domain.AdminContextInput{}, err
	}
	tokenID, err := optionalStringClaim(claims, "jti")
	if err != nil {
		return domain.AdminContextInput{}, err
	}

	return domain.AdminContextInput{
		RequestID:    requestID,
		AdminID:      adminID,
		Roles:        roles,
		TenantScopes: tenantScopes,
		Permissions:  permissions,
		TokenID:      tokenID,
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

func requiredStringSliceClaim(claims jwtClaims, key string) ([]string, error) {
	value, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf("jwt claim %q is required", key)
	}
	return decodeStringSliceClaim(value, key)
}

func optionalStringSliceClaim(claims jwtClaims, key string) ([]string, error) {
	value, ok := claims[key]
	if !ok {
		return nil, nil
	}
	return decodeStringSliceClaim(value, key)
}

func decodeStringSliceClaim(value json.RawMessage, key string) ([]string, error) {
	var result []string
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("jwt claim %q must be a string array", key)
	}
	return result, nil
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
	return multiple, nil
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
