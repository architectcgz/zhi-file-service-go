package runtime

import (
	"testing"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/bootstrap"
)

func TestBuildRejectsNilApp(t *testing.T) {
	t.Parallel()

	if _, err := Build(nil); err == nil {
		t.Fatal("Build() error = nil, want non-nil")
	}
}

func TestBuildRejectsMissingDatabase(t *testing.T) {
	t.Parallel()

	if _, err := Build(&bootstrap.App{}); err == nil {
		t.Fatal("Build() error = nil, want non-nil")
	}
}
