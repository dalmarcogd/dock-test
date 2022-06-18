package accountsh

import (
	"net/http"

	"github.com/dalmarcogd/dock-test/internal/accounts"
	"github.com/dalmarcogd/dock-test/pkg/zapctx"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type (
	UnblockByIDFunc echo.HandlerFunc

	unblockByID struct {
		ID string `param:"id"`
	}
)

func NewUnblockByIDFunc(svc accounts.Service) UnblockByIDFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		var cls unblockByID
		if err := c.Bind(&cls); err != nil {
			zapctx.L(ctx).Error("unblock_by_account_id_handler_bind_error", zap.Error(err))
			return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
		}

		id, err := uuid.Parse(cls.ID)
		if err != nil {
			zapctx.L(ctx).Error("unblock_by_account_id_handler_bind_error", zap.Error(err))
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "invalid id")
		}

		account, err := svc.UnblockByID(ctx, id)
		if err != nil {
			zapctx.L(ctx).Error("unblock_by_account_id_handler_service_error", zap.Error(err))
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		return c.JSON(
			http.StatusOK,
			createdAccount{
				ID:             account.ID.String(),
				Name:           account.Name,
				Agency:         account.Agency,
				Number:         account.Number,
				DocumentNumber: account.DocumentNumber,
				Status:         string(account.Status),
			},
		)
	}
}
