package testkit

import "testing"

func SetEnv(t *testing.T, key string, value string) {
	t.Helper()
	t.Setenv(key, value)
}
