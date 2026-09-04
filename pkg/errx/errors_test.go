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

// The With* builders must not mutate the receiver: constructors are frequently
// called inline and their results passed around, and a shared base error that
// picked up another call site's message would leak information across requests.
func TestWithBuildersCloneRatherThanMutate(t *testing.T) {
	base := NotFound().WithData(map[string]any{"resource": "order"})

	derived := base.
		WithMessage("订单不存在").
		WithCode("order_not_found").
		WithHTTPStatus(http.StatusGone).
		WithDataValue("order_id", "o-1").
		WithCause(stderrors.New("row missing"))

	if base.Message == "订单不存在" || base.Code == "order_not_found" {
		t.Fatalf("base error was mutated: %+v", base)
	}
	if base.HTTPStatus != http.StatusNotFound {
		t.Fatalf("base status = %d, want 404", base.HTTPStatus)
	}
	if _, exists := base.Data["order_id"]; exists {
		t.Fatalf("WithDataValue mutated the base error's data: %+v", base.Data)
	}
	if base.Cause != nil {
		t.Fatal("WithCause mutated the base error")
	}

	if derived.Message != "订单不存在" || derived.Code != "order_not_found" {
		t.Fatalf("derived error = %+v", derived)
	}
	if derived.HTTPStatus != http.StatusGone {
		t.Fatalf("derived status = %d, want 410", derived.HTTPStatus)
	}
	if derived.Data["order_id"] != "o-1" || derived.Data["resource"] != "order" {
		t.Fatalf("derived data = %+v, want both the inherited and the new value", derived.Data)
	}
}

// Clone must deep-copy Data, otherwise two errors share one map and a write
// through either is visible in both.
func TestCloneDoesNotShareTheDataMap(t *testing.T) {
	original := Conflict().WithData(map[string]any{"field": "email"})
	cloned := original.Clone()
	cloned.Data["field"] = "phone"

	if original.Data["field"] != "email" {
		t.Fatalf("clone shares the data map: original = %+v", original.Data)
	}
	if Clone := (*HTTPError)(nil).Clone(); Clone != nil {
		t.Fatal("cloning nil must stay nil")
	}
}

func TestErrorMessageFallsBackToTheStatusText(t *testing.T) {
	if got := NotFound().WithMessage("").Error(); got != http.StatusText(http.StatusNotFound) {
		t.Fatalf("empty message error = %q, want the 404 status text", got)
	}
	if got := InvalidParams().WithMessage("参数错误").Error(); got != "参数错误" {
		t.Fatalf("error = %q, want the message", got)
	}
	if got := (*HTTPError)(nil).Error(); got != "" {
		t.Fatalf("nil error string = %q, want empty", got)
	}
}

// The cause stays in the error chain so services can match with errors.Is
// without importing this package.
func TestCauseRemainsReachableThroughErrorsIs(t *testing.T) {
	sentinel := stderrors.New("upstream down")
	wrapped := ServiceUnavailable().WithCause(sentinel)

	if !stderrors.Is(wrapped, sentinel) {
		t.Fatal("errors.Is could not reach the cause")
	}
	if stderrors.Unwrap(wrapped) != sentinel {
		t.Fatal("Unwrap did not return the cause")
	}
	if (*HTTPError)(nil).Unwrap() != nil {
		t.Fatal("unwrapping nil must be nil")
	}
}

// A status outside the HTTP range would make net/http emit an invalid response.
func TestEffectiveStatusAndCodeClampMalformedValues(t *testing.T) {
	for _, status := range []int{0, -1, 99, 600, 1000} {
		if got := EffectiveStatus(&HTTPError{HTTPStatus: status}); got != http.StatusInternalServerError {
			t.Errorf("EffectiveStatus(%d) = %d, want 500", status, got)
		}
	}
	if got := EffectiveStatus(nil); got != http.StatusInternalServerError {
		t.Errorf("EffectiveStatus(nil) = %d, want 500", got)
	}
	if got := EffectiveStatus(&HTTPError{HTTPStatus: http.StatusTeapot}); got != http.StatusTeapot {
		t.Errorf("EffectiveStatus(418) = %d, want 418", got)
	}

	if got := EffectiveCode(nil); got != "" {
		t.Errorf("EffectiveCode(nil) = %q, want empty", got)
	}
	if got := EffectiveCode(&HTTPError{Code: "  custom  "}); got != "custom" {
		t.Errorf("EffectiveCode trimmed = %q, want custom", got)
	}
	// No code and an unmappable status still needs something stable.
	if got := EffectiveCode(&HTTPError{HTTPStatus: 0}); got == "" {
		t.Error("EffectiveCode must never be empty for a non-nil error")
	}
}

func TestConstructorsCarryTheirStatus(t *testing.T) {
	// InvalidParams is deliberately 400 as well; callers that want 422 set it
	// explicitly with WithHTTPStatus.
	cases := map[string]struct {
		err  *HTTPError
		want int
	}{
		"BadRequest":         {BadRequest(), http.StatusBadRequest},
		"InvalidParams":      {InvalidParams(), http.StatusBadRequest},
		"Unauthorized":       {Unauthorized(), http.StatusUnauthorized},
		"Forbidden":          {Forbidden(), http.StatusForbidden},
		"NotFound":           {NotFound(), http.StatusNotFound},
		"Conflict":           {Conflict(), http.StatusConflict},
		"TooManyRequests":    {TooManyRequests(), http.StatusTooManyRequests},
		"ServiceUnavailable": {ServiceUnavailable(), http.StatusServiceUnavailable},
		"Internal":           {Internal(), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		if tc.err.HTTPStatus != tc.want {
			t.Errorf("%s returned status %d, want %d", name, tc.err.HTTPStatus, tc.want)
		}
		if EffectiveCode(tc.err) == "" {
			t.Errorf("%s has no effective code", name)
		}
	}

	// The upload limit is reported to the client, so it has to survive.
	tooLarge := RequestBodyTooLarge(2048)
	// A non-positive limit carries no field rather than an misleading zero.
	if _, exists := RequestBodyTooLarge(0).Data["max_bytes"]; exists {
		t.Error("RequestBodyTooLarge(0) must not report a limit")
	}
	if tooLarge.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Errorf("RequestBodyTooLarge status = %d", tooLarge.HTTPStatus)
	}
	if tooLarge.Data["max_bytes"] != int64(2048) {
		t.Errorf("RequestBodyTooLarge data = %+v, want the limit", tooLarge.Data)
	}
}
