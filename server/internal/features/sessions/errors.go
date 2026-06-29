package sessions

import "errors"

var (
	ErrTokenRequired        = errors.New("session_token is required")
	ErrMachineRequired      = errors.New("machine_id is required")
	ErrCardRequired         = errors.New("card is required")
	ErrCardNotFound         = errors.New("card not found")
	ErrCardBlacklisted      = errors.New("card blacklisted")
	ErrCardDisabled         = errors.New("card disabled")
	ErrCardExpired          = errors.New("card expired")
	ErrCardNoLongerValid    = errors.New("card no longer valid")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("invalid or expired session")
	ErrMachineMismatch      = errors.New("session machine mismatch")
	ErrCardMismatch         = errors.New("session card mismatch")
	ErrMaxSessionsReached   = errors.New("max sessions reached")
	ErrBoundToOtherMachine  = errors.New("card already bound to another machine")
)
