package transactionsh

import (
	"errors"
	"net/http"

	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/stringers"
	"github.com/dalmarcogd/dock-test/internal/transactions"
	"github.com/dalmarcogd/dock-test/pkg/zapctx"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type (
	CreateTransactionFunc echo.HandlerFunc

	createTransaction struct {
		From        string  `json:"from_account_id"`
		To          string  `json:"to_account_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
	createdTransaction struct {
		ID          string  `json:"id"`
		From        string  `json:"from_account_id,omitempty"`
		To          string  `json:"to_account_id,omitempty"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
	}
)

func NewCreateTransactionFunc(svc transactions.Service) CreateTransactionFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		var trx createTransaction
		err := c.Bind(&trx)
		if err != nil {
			zapctx.L(ctx).Error("create_transaction_handler_bind_error", zap.Error(err))
			return err
		}

		var fromID uuid.UUID
		if trx.From != "" {
			fromID, err = uuid.Parse(trx.From)
			if err != nil {
				zapctx.L(ctx).Error("create_transaction_handler_parse_error", zap.Error(err))
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "invalid from account id")
			}
		}

		var toID uuid.UUID
		if trx.To != "" {
			toID, err = uuid.Parse(trx.To)
			if err != nil {
				zapctx.L(ctx).Error("create_transaction_handler_parse_error", zap.Error(err))
				return echo.NewHTTPError(http.StatusUnprocessableEntity, "invalid to account id")
			}
		}

		transaction, err := svc.Create(ctx, transactions.Transaction{
			From:        fromID,
			To:          toID,
			Amount:      trx.Amount,
			Description: trx.Description,
		})
		if err != nil {
			zapctx.L(ctx).Error("create_transaction_handler_service_error", zap.Error(err))
			if errors.Is(err, transactions.ErrBalanceInsufficientFunds) {
				return echo.NewHTTPError(http.StatusPreconditionFailed, err.Error())
			} else if errors.Is(err, transactions.ErrFromAccountNotfound) || errors.Is(err, transactions.ErrToAccountNotfound) {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			} else if errors.Is(err, transactions.ErrFromAccountToAccountShouldBeDifferent) {
				return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
			}

			return echo.NewHTTPError(http.StatusBadRequest, "was not possible to register the transaction")
		}

		return c.JSON(
			http.StatusCreated,
			createdTransaction{
				ID:          stringers.UUIDEmpty(transaction.ID),
				From:        stringers.UUIDEmpty(transaction.From),
				To:          stringers.UUIDEmpty(transaction.To),
				Amount:      transaction.Amount,
				Description: transaction.Description,
			},
		)
	}
}