package domainerr

// Code is a stable, transport-independent domain error code.
type Code string

const (
	CodeValidationFailed    Code = "validation_failed"
	CodeUnauthenticated     Code = "unauthenticated"
	CodeForbidden           Code = "forbidden"
	CodeNotFound            Code = "not_found"
	CodeMethodNotAllowed    Code = "method_not_allowed"
	CodeRevisionConflict    Code = "revision_conflict"
	CodeIdempotencyConflict Code = "idempotency_conflict"
	CodeRateLimited         Code = "rate_limited"
	CodeTimeout             Code = "timeout"
	CodeInternalError       Code = "internal_error"
)

var catalog = []struct {
	Code      Code
	Retryable bool
}{
	{CodeValidationFailed, false},
	{CodeUnauthenticated, false},
	{CodeForbidden, false},
	{CodeNotFound, false},
	{CodeMethodNotAllowed, false},
	{CodeRevisionConflict, true},
	{CodeIdempotencyConflict, false},
	{CodeRateLimited, true},
	{CodeTimeout, true},
	{CodeInternalError, true},
}

// Codes returns the stable catalog in documented order.
func Codes() []Code {
	out := make([]Code, len(catalog))
	for i, e := range catalog {
		out[i] = e.Code
	}
	return out
}

// Retryable reports the catalog default for code. Unknown codes are not retryable.
func Retryable(code Code) bool {
	for _, e := range catalog {
		if e.Code == code {
			return e.Retryable
		}
	}
	return false
}
