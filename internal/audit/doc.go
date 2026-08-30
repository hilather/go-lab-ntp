// Package audit records mutation and security events.
//
// First GA keeps a bounded in-memory ring and an optional external hook.
// Hook delivery failure is counted and never fail-closes the mutation.
package audit
