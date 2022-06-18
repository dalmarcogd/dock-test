package balances

import (
	"context"
	"database/sql"
	"errors"

	"github.com/dalmarcogd/dock-test/pkg/tracer"
	"github.com/dalmarcogd/dock-test/pkg/zapctx"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service interface {
	GetByAccountID(ctx context.Context, accountID uuid.UUID) (AccountBalance, error)
}

type service struct {
	tracer     tracer.Tracer
	repository Repository
}

func NewService(t tracer.Tracer, r Repository) Service {
	return service{tracer: t, repository: r}
}

func (s service) GetByAccountID(ctx context.Context, accountID uuid.UUID) (AccountBalance, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	accountBalance, err := s.repository.GetByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccountBalance{
				AccountID:      accountID,
				CurrentBalance: 0,
			}, nil
		}
		zapctx.L(ctx).Error("balances_service_repository_error", zap.Error(err))
		span.RecordError(err)
		return AccountBalance{}, err
	}

	return AccountBalance{
		AccountID:      accountBalance.AccountID,
		CurrentBalance: accountBalance.Balance,
	}, nil
}
