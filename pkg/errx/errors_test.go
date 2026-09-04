package errx

import (
	stderrors "errors"
	"net/http"
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

func TestHTTPErrorUnwrapsCause(t *testing.T) {
	cause := stderrors.New("database unavailable")
	err := ServiceUnavailable().WithCause(cause)
	if !stderrors.Is(err, cause) {
		t.Fatal("expected HTTPError to expose its cause to errors.Is")
	}
}

func TestWithDataCopiesMap(t *testing.T) {
	data := map[string]any{"error_code": "one"}
	err := Conflict().WithData(data)
	data["error_code"] = "two"
	if got := err.Data["error_code"]; got != "one" {
		t.Fatalf("WithData aliased caller map: %#v", got)
	}

	err.Data["error_code"] = "three"
	if got := data["error_code"]; got != "two" {
		t.Fatalf("WithData result aliased caller map: %#v", got)
	}
}

func TestEffectiveCodeAndStatusCloseCustomErrorContract(t *testing.T) {
	err := New(0, "", "bad")
	if got := EffectiveStatus(err); got != http.StatusInternalServerError {
		t.Fatalf("unexpected effective status: %d", got)
	}
	if got := EffectiveCode(err); got != "internal_error" {
		t.Fatalf("unexpected effective code: %s", got)
	}
}

func TestNormalizeTypedNilHTTPErrorDoesNotDropFailure(t *testing.T) {
	var typedNil *HTTPError
	var err error = typedNil
	normalized := Normalize(err)
	if normalized == nil || normalized.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("expected internal error for typed nil, got %#v", normalized)
	}
}
