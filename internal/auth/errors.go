package auth

import (
	"errors"
	"strings"

	"github.com/hilather/go-lab-controlkit/authn"
	"github.com/hilather/go-lab-controlkit/kerr"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
)

// mapLoadErr renders ntp validation text from a kit load failure.
// Kit Msg is not copied: the wire sentences are the ones FromSpec used
// before the facade. An unrecognized *authn.LoadError is a generic
// validation failure and does not claim the secret file is missing.
// Any other error is a neutral ntp sentence, not the kit's "authn:" text.
func mapLoadErr(err error) error {
	if err == nil {
		return nil
	}
	var le *authn.LoadError
	if !errors.As(err, &le) || le == nil {
		return domainerr.ValidationFailed("token configuration is invalid")
	}
	field := le.Field
	leaf := field
	if i := strings.LastIndex(field, "."); i >= 0 {
		leaf = field[i+1:]
	}
	switch {
	case le.Code == "invalid_value" && leaf == "mode":
		return domainerr.ValidationFailed("unknown auth mode",
			viol(field, "invalid_value", "unknown auth mode"))
	case le.Code == "empty_id":
		return domainerr.ValidationFailed("token id is required",
			viol(field, "empty_id", "token id is required"))
	case le.Code == "duplicate_id" && leaf == "id":
		return domainerr.ValidationFailed("duplicate token id",
			viol(field, "duplicate_id", "duplicate token id"))
	case le.Code == "invalid_value" && leaf == "secretFile":
		return domainerr.ValidationFailed("token entropy is below 256 bits",
			viol(field, "invalid_value", "token secret must be at least 32 bytes"))
	case le.Code == "duplicate_id" && leaf == "secretFile":
		msg := le.Msg
		if !strings.HasPrefix(msg, "token value matches ") {
			msg = "token value matches " + le.TokenID
		}
		return domainerr.ValidationFailed("duplicate token value",
			viol(field, "duplicate_id", msg))
	case le.Code == "invalid_value" && leaf == "role":
		return domainerr.ValidationFailed("unknown role",
			viol(field, "invalid_value", "role must be viewer, operator, or administrator"))
	case le.Code == "unresolved_reference":
		return domainerr.ValidationFailed("token secret is unavailable",
			viol(field, "unresolved_reference", "token secret file does not resolve"))
	default:
		return domainerr.ValidationFailed("validation failed")
	}
}

func viol(path, code, message string) domainerr.FieldViolation {
	return domainerr.FieldViolation{Path: path, Code: code, Message: message}
}

// mapUnauth turns every kit authentication failure, including "invalid token",
// into the ntp sentence.
func mapUnauth(err error) error {
	if err == nil {
		return nil
	}
	if de, ok := domainerr.As(err); ok {
		return de
	}
	return domainerr.Unauthenticated("authentication required")
}

// mapSessionMint turns the kit mint failure "session: entropy unavailable"
// (kerr.Internal) into "session material unavailable".
func mapSessionMint(err error) error {
	if err == nil {
		return nil
	}
	if de, ok := domainerr.As(err); ok {
		return de
	}
	if kind, ok := kerr.KindOf(err); ok && kind == kerr.Internal {
		return domainerr.Internal("session material unavailable")
	}
	return domainerr.Internal("session material unavailable")
}
