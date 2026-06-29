package sessions

import (
	"context"
	"time"
)

type Store interface {
	RunInTx(ctx context.Context, fn func(Tx) error) error
	GetSession(ctx context.Context, token string) (Session, error)
	ListSessions(ctx context.Context, filter ListFilter, now time.Time) ([]Session, int, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

type Tx interface {
	GetCard(ctx context.Context, code string) (CardSnapshot, error)
	IsCardBlacklisted(ctx context.Context, code string) (bool, error)
	ListSessionsByCard(ctx context.Context, code string) ([]Session, error)
	InsertSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, token string) (Session, error)
	UpdateSession(ctx context.Context, session Session) error
	DeleteSession(ctx context.Context, token string) error
	UpdateCardActivation(ctx context.Context, card CardSnapshot) error
	UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error
}
