package transactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dalmarcogd/dock-test/internal/accounts"
	"github.com/dalmarcogd/dock-test/internal/balances"
	"github.com/dalmarcogd/dock-test/pkg/distlock"
	"github.com/dalmarcogd/dock-test/pkg/redis"
	"github.com/dalmarcogd/dock-test/pkg/tracer"
	"github.com/dalmarcogd/dock-test/pkg/zapctx"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrTransactionNotFound                   = errors.New("no transaction found with these filters")
	ErrFailLockAccount                       = errors.New("was not possible to lock account to process the operation")
	ErrMultpleTransactionsFound              = errors.New("multiple transactions found with these filters")
	ErrFromAccountToAccountShouldBeDifferent = errors.New("the from account and to account should not be equal")
	ErrFromAccountNotfound                   = errors.New("the from account could be found")
	ErrToAccountNotfound                     = errors.New("the to account could be found")
	ErrGetAccountBalance                     = errors.New("received error when get the account balance")
	ErrBalanceInsufficientFunds              = errors.New("insufficient funds to complete the transaction")
	ErrAccountInactive                       = errors.New("the account involved in the transaction must be active")
)

type Service interface {
	CreateCredit(ctx context.Context, transaction Transaction) (Transaction, error)
	CreateDebit(ctx context.Context, transaction Transaction) (Transaction, error)
	CreateP2P(ctx context.Context, transaction Transaction) (Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (Transaction, error)
}

type service struct {
	tracer      tracer.Tracer
	repository  Repository
	locker      distlock.DistLock
	accountsSvs accounts.Service
	balancesSvs balances.Service
	redis       redis.Client
}

func NewService(
	t tracer.Tracer,
	r Repository,
	l distlock.DistLock,
	as accounts.Service,
	bs balances.Service,
	redis redis.Client,
) Service {
	return service{
		tracer:      t,
		repository:  r,
		locker:      l,
		accountsSvs: as,
		balancesSvs: bs,
		redis:       redis,
	}
}

func (s service) CreateCredit(ctx context.Context, transaction Transaction) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	transaction.Type = CreditTransaction

	err := s.checkAccount(ctx, transaction.To)
	if err != nil {
		span.RecordError(err)
		return Transaction{}, err
	}

	return s.createCredit(ctx, transaction)
}

func (s service) CreateDebit(ctx context.Context, transaction Transaction) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	transaction.Type = DebitTransaction

	err := s.checkAccount(ctx, transaction.From)
	if err != nil {
		span.RecordError(err)
		return Transaction{}, err
	}

	return s.createDebit(ctx, transaction)
}

func (s service) CreateP2P(ctx context.Context, transaction Transaction) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	transaction.Type = P2PTransaction

	if transaction.From == transaction.To {
		zapctx.L(ctx).Error(
			"transaction_service_from_acccount_to_account_equal_error",
			zap.Error(ErrFromAccountToAccountShouldBeDifferent),
			zap.String("from", transaction.From.String()),
			zap.String("to", transaction.To.String()),
		)
		span.RecordError(ErrFromAccountToAccountShouldBeDifferent)
		return Transaction{}, ErrFromAccountToAccountShouldBeDifferent
	}

	err := s.checkAccount(ctx, transaction.From)
	if err != nil {
		span.RecordError(err)
		return Transaction{}, err
	}

	err = s.checkAccount(ctx, transaction.To)
	if err != nil {
		span.RecordError(err)
		return Transaction{}, err
	}

	return s.createDebit(ctx, transaction)
}

func (s service) checkAccount(ctx context.Context, accountID uuid.UUID) error {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	toAccount, err := s.accountsSvs.GetByID(ctx, accountID)
	if err != nil {
		span.RecordError(err)

		if !errors.Is(err, accounts.ErrAccountNotFound) {
			zapctx.L(ctx).Error(
				"transaction_service_acccount_check_error",
				zap.Error(err),
				zap.String("account_id", accountID.String()),
			)
		}
		return ErrToAccountNotfound
	}

	if toAccount.Status != accounts.ActiveStatus {
		zapctx.L(ctx).Error(
			"transaction_service_acccount_inactive_error",
			zap.Error(ErrAccountInactive),
			zap.String("account_id", accountID.String()),
		)
		span.RecordError(ErrAccountInactive)
		return ErrAccountInactive
	}

	return nil
}

func (s service) createDebit(ctx context.Context, transaction Transaction) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	transactionAccountLockerKey := fmt.Sprintf("transaction-account-from-%s", transaction.From.String())
	defer s.locker.Release(ctx, transactionAccountLockerKey)

	if s.locker.Acquire(
		ctx,
		transactionAccountLockerKey,
		50*time.Millisecond,
		3,
	) {
		accountBalance, err := s.balancesSvs.GetByAccountID(ctx, transaction.From)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			zapctx.L(ctx).Error("transaction_service_get_balance_error", zap.Error(err))
			span.RecordError(err)
			return Transaction{}, ErrGetAccountBalance
		}

		if (accountBalance.CurrentBalance - transaction.Amount) >= 0 {
			model, err := s.repository.Create(ctx, newTransactionModel(transaction))
			if err != nil {
				zapctx.L(ctx).Error("transaction_service_create_repository_error", zap.Error(err))
				span.RecordError(err)
				return Transaction{}, err
			}

			transaction.ID = model.ID
			return transaction, nil
		} else {
			return Transaction{}, ErrBalanceInsufficientFunds
		}
	}

	return Transaction{}, ErrFailLockAccount
}

func (s service) createCredit(ctx context.Context, transaction Transaction) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	model, err := s.repository.Create(ctx, newTransactionModel(transaction))
	if err != nil {
		zapctx.L(ctx).Error("transaction_service_create_repository_error", zap.Error(err))
		span.RecordError(err)
		return Transaction{}, err
	}

	transaction.ID = model.ID

	return transaction, nil
}

func (s service) GetByID(ctx context.Context, id uuid.UUID) (Transaction, error) {
	ctx, span := s.tracer.Span(ctx)
	defer span.End()

	models, err := s.repository.GetByFilter(ctx, transactionFilter{ID: uuid.NullUUID{UUID: id, Valid: true}})
	if err != nil {
		zapctx.L(ctx).Error(
			"transaction_service_get_repository_error",
			zap.String("id", id.String()),
			zap.Error(err),
		)
		span.RecordError(err)
		return Transaction{}, err
	}

	if len(models) == 0 {
		return Transaction{}, ErrTransactionNotFound
	}

	if len(models) > 1 {
		return Transaction{}, ErrMultpleTransactionsFound
	}

	return newTransaction(models[0]), nil
}
