package domainerr

import "errors"

// FieldViolation is one structured validation path.
type FieldViolation struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is the public domain error. It never includes secrets or stack traces.
type Error struct {
	Code            Code             `json:"code"`
	Message         string           `json:"message"`
	Retryable       bool             `json:"retryable"`
	FieldViolations []FieldViolation `json:"fieldViolations,omitempty"`
	CurrentRevision string           `json:"currentRevision,omitempty"`
	Remediation     string           `json:"remediation,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Message
}

// Is reports whether target is an *Error with the same Code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e != nil && t != nil && e.Code == t.Code
}

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// New returns an error for code with the catalog retryable default.
func New(code Code, message string) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Retryable: Retryable(code),
	}
}

func (e *Error) clone() *Error {
	if e == nil {
		return nil
	}
	c := *e
	if e.FieldViolations != nil {
		c.FieldViolations = append([]FieldViolation(nil), e.FieldViolations...)
	}
	return &c
}

// WithRemediation returns a copy with a safe operator/agent hint.
func (e *Error) WithRemediation(s string) *Error {
	c := e.clone()
	if c != nil {
		c.Remediation = s
	}
	return c
}

// WithRevision returns a copy that records the current content revision.
func (e *Error) WithRevision(rev string) *Error {
	c := e.clone()
	if c != nil {
		c.CurrentRevision = rev
	}
	return c
}

// WithViolations returns a copy with additional field violations.
func (e *Error) WithViolations(v ...FieldViolation) *Error {
	if e == nil {
		return nil
	}
	if len(v) == 0 {
		return e
	}
	c := e.clone()
	c.FieldViolations = append(c.FieldViolations, v...)
	return c
}

func ValidationFailed(message string, violations ...FieldViolation) *Error {
	return New(CodeValidationFailed, message).WithViolations(violations...)
}

func RevisionConflict(message, currentRevision string) *Error {
	return New(CodeRevisionConflict, message).WithRevision(currentRevision)
}

func IdempotencyConflict(message string) *Error {
	return New(CodeIdempotencyConflict, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, message)
}

func MethodNotAllowed(message string) *Error {
	return New(CodeMethodNotAllowed, message)
}

func Forbidden(message string) *Error {
	return New(CodeForbidden, message)
}

func Unauthenticated(message string) *Error {
	return New(CodeUnauthenticated, message)
}

func RateLimited(message string) *Error {
	return New(CodeRateLimited, message)
}

func Timeout(message string) *Error {
	return New(CodeTimeout, message)
}

func Internal(message string) *Error {
	return New(CodeInternalError, message)
}
