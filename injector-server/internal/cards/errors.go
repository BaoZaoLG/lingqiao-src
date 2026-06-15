package cards

import "errors"

// Sentinel errors for card and session operations.
// Callers should use errors.Is() to match these.
var (
	ErrCardNotFound            = errors.New("card not found")
	ErrCardBlacklisted         = errors.New("card is blacklisted")
	ErrCardDisabled            = errors.New("card is disabled")
	ErrCardExpired             = errors.New("card has expired")
	ErrCardBoundToOtherMachine = errors.New("card already bound to another machine")
	ErrSessionExpired          = errors.New("session expired")
	ErrSessionNotFound         = errors.New("session not found")
	ErrCardNoLongerValid       = errors.New("card no longer valid")
	ErrMaxSessionsReached      = errors.New("max active sessions reached")
	ErrMachineMismatch         = errors.New("machine mismatch")
)
