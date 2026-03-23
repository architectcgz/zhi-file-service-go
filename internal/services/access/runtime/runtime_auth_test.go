package runtime

import (
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
)

func TestBuildRejectsInvalidJWKSConfig(t *testing.T) {
	t.Parallel()

	app := &bootstrap.App{
		Config: config.Config{
			Storage: runtimeStorageConfig(),
			Access: config.AccessConfig{
				AuthJWKS: "{not-json",
			},
		},
		DB:      &sql.DB{},
		Storage: runtimeStorageClient(t),
	}

	if _, err := Build(app); err == nil {
		t.Fatal("Build() error = nil, want non-nil")
	}
}

func TestBuildAcceptsInlineJWKSConfig(t *testing.T) {
	t.Parallel()

	app := &bootstrap.App{
		Config: config.Config{
			Storage: runtimeStorageConfig(),
			Access: config.AccessConfig{
				AuthJWKS:         runtimeTestJWKS(t),
				TicketSigningKey: "ticket-key",
			},
		},
		DB:      &sql.DB{},
		Storage: runtimeStorageClient(t),
	}

	options, err := Build(app)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if options.Handler == nil {
		t.Fatal("Build() handler = nil, want non-nil")
	}
}

func runtimeStorageClient(t *testing.T) *platformstorage.Client {
	t.Helper()

	client, err := platformstorage.Open(runtimeStorageConfig())
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	return client
}

func runtimeStorageConfig() config.StorageConfig {
	return config.StorageConfig{
		Endpoint:      "http://127.0.0.1:9000",
		Region:        "us-east-1",
		AccessKey:     "key",
		SecretKey:     "secret",
		PublicBucket:  "public",
		PrivateBucket: "private",
	}
}

func runtimeTestJWKS(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": "runtime-key",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(payload)
}
