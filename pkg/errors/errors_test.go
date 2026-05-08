package errors

import (
	stderrors "errors"
	"testing"
)

func TestNormalizeFindsWrappedHTTPError(t *testing.T) {
	httpErr := Forbidden().WithMessage("denied")
	wrapped := stderrors.Join(stderrors.New("audit failed"), httpErr)

	normalized := Normalize(wrapped)
	if normalized != httpErr {
		t.Fatalf("expected wrapped HTTPError, got %#v", normalized)
	}
}
