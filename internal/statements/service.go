package statements

import (
	"context"

	"github.com/dalmarcogd/dock-test/internal/accounts"
	"github.com/dalmarcogd/dock-test/pkg/tracer"
	"github.com/dalmarcogd/dock-test/pkg/zapctx"
	"go.uber.org/zap"
)

type Service interface {
	List(ctx context.Context, filter ListFilter) (int, []Statement, error)
}

type service struct {
	tracer     tracer.Tracer
	repository Repository
}

func NewService(t tracer.Tracer, r Repository) Service {
	return service{tracer: t, repository: r}
}

func (s service) List(ctx context.Context, filter ListFilter) (int, []Statement, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	total, statementModels, err := s.repository.ListByFilter(ctx, StatementFilter{
		Page:           filter.Page,
		Size:           filter.Size,
		Sort:           filter.Sort,
		AccountID:      filter.AccountID,
		CreatedAtBegin: filter.CreatedAtBegin,
		CreatedAtEnd:   filter.CreatedAtEnd,
	})
	if err != nil {
		zapctx.L(ctx).Error("statements_service_repository_error", zap.Error(err))
		span.RecordError(err)
		return 0, []Statement{}, err
	}

	stmts := make([]Statement, len(statementModels))
	for i, model := range statementModels {
		stmts[i] = Statement{
			FromAccount: accounts.Account{
				ID:   model.FromAccountID,
				Name: model.FromAccountName,
			},
			ToAccount: accounts.Account{
				ID:   model.ToAccountID,
				Name: model.ToAccountName,
			},
			Type:        model.Type,
			Amount:      model.Amount,
			Description: model.Description,
			CreatedAt:   model.CreatedAt,
		}
	}

	return total, stmts, nil
}
