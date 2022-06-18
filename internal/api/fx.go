package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/dalmarcogd/dock-test/internal/accounts"
	"github.com/dalmarcogd/dock-test/internal/api/internal/environment"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/accountsh"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/balancesh"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/holdersh"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/statementsh"
	"github.com/dalmarcogd/dock-test/internal/api/internal/handlers/transactionsh"
	"github.com/dalmarcogd/dock-test/internal/balances"
	"github.com/dalmarcogd/dock-test/internal/holders"
	"github.com/dalmarcogd/dock-test/internal/statements"
	"github.com/dalmarcogd/dock-test/internal/transactions"
	"github.com/dalmarcogd/dock-test/pkg/database"
	"github.com/dalmarcogd/dock-test/pkg/distlock"
	"github.com/dalmarcogd/dock-test/pkg/healthcheck"
	"github.com/dalmarcogd/dock-test/pkg/http/middlewares"
	"github.com/dalmarcogd/dock-test/pkg/redis"
	"github.com/dalmarcogd/dock-test/pkg/tracer"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Options(
	// Infra
	fx.Provide(
		environment.NewEnvironment,
		func(lc fx.Lifecycle, e environment.Environment, t tracer.Tracer) (database.Database, error) {
			return database.Setup(lc, t, e.DatabaseURL, e.DatabaseURL)
		},
		func(env environment.Environment) (redis.Client, error) {
			return redis.NewClient(env.RedisURL, env.RedisCACert)
		},
		func(env environment.Environment, db database.Database, redisClient redis.Client) healthcheck.HealthCheck {
			return healthcheck.NewChain(
				healthcheck.NewDatabaseConnectivity(db.Master()),
				healthcheck.NewDatabaseMigration(db.Master(), "schema_migrations"),
				healthcheck.NewDatabaseConnectivity(db.Replica()),
				healthcheck.NewDatabaseMigration(db.Replica(), "schema_migrations"),
				redis.NewHealthCheck(redisClient),
			)
		},
		func(lc fx.Lifecycle, e environment.Environment) (tracer.Tracer, error) {
			return tracer.Setup(lc, e.OtelCollectorHost, e.Service, e.Environment, e.Version)
		},
		distlock.NewDistock,
	),
	// Domains
	fx.Provide(
		holders.NewRepository,
		holders.NewService,
		accounts.NewRepository,
		accounts.NewService,
		transactions.NewRepository,
		transactions.NewService,
		statements.NewRepository,
		statements.NewService,
		balances.NewRepository,
		balances.NewService,
	),
	// Endpoints
	fx.Provide(
		handlers.NewLivenessFunc,
		handlers.NewReadinessFunc,
		holdersh.NewCreateHolderFunc,
		holdersh.NewGetByIDHolderFunc,
		holdersh.NewListHoldersFunc,
		accountsh.NewCreateAccountFunc,
		accountsh.NewBlockByIDFunc,
		accountsh.NewUnblockByIDFunc,
		accountsh.NewCloseByIDFunc,
		accountsh.NewGetByIDFunc,
		accountsh.NewListAccountsFunc,
		statementsh.NewListAccountStatementFunc,
		balancesh.NewGetBalanceByAccountIDFunc,
		transactionsh.NewCreateCreditTransactionFunc,
		transactionsh.NewCreateDebitTransactionFunc,
		transactionsh.NewCreateP2PTransactionFunc,
		transactionsh.NewGetByIDTransactionFunc,
	),
	// Startup applications
	fx.Invoke(func(
		env environment.Environment,
	) (*zap.Logger, error) {
		return setupLogger(
			env.Service,
			env.Version,
			env.Environment,
		)
	}),
	fx.Invoke(runHTTPServer),
)

func setupLogger(service, version, env string) (*zap.Logger, error) {
	logger := zap.L().With(
		zap.String("service", service),
		zap.String("version", version),
		zap.String("env", env),
		// dd is prefix to DataDog (our software for APN of Hash services)
	)
	_ = zap.ReplaceGlobals(logger)
	return logger, nil
}

//nolint:funlen
func runHTTPServer(
	lc fx.Lifecycle,
	env environment.Environment,
	t tracer.Tracer,
	readinessFunc handlers.ReadinessFunc,
	livenessFunc handlers.LivenessFunc,
	createHolderFunc holdersh.CreateHolderFunc,
	getByIDHolderFunc holdersh.GetByIDHolderFunc,
	listHoldersFunc holdersh.ListHoldersFunc,
	createAccountFunc accountsh.CreateAccountFunc,
	closeByIDFunc accountsh.CloseByIDFunc,
	blockByIDFunc accountsh.BlockByIDFunc,
	unblockByIDFunc accountsh.UnblockByIDFunc,
	getByIDAccountFunc accountsh.GetByIDFunc,
	listAccountsFunc accountsh.ListAccountsFunc,
	createCreditTransactionFunc transactionsh.CreateCreditTransactionFunc,
	createDebitTransactionFunc transactionsh.CreateDebitTransactionFunc,
	createP2PTransactionFunc transactionsh.CreateP2PTransactionFunc,
	getByIDTransactionFunc transactionsh.GetByIDTransactionFunc,
	listAccountStatementFunc statementsh.ListAccountStatementFunc,
	getBalanceByIDAccountFunc balancesh.GetBalanceByAccountIDFunc,
) error {
	e := echo.New()

	e.GET("/readiness", echo.HandlerFunc(readinessFunc))
	e.GET("/liveness", echo.HandlerFunc(livenessFunc))
	v1 := e.Group("/v1")
	v1.POST("/holders", echo.HandlerFunc(createHolderFunc))
	v1.GET("/holders/:id", echo.HandlerFunc(getByIDHolderFunc))
	v1.GET("/holders", echo.HandlerFunc(listHoldersFunc))
	v1.POST("/accounts", echo.HandlerFunc(createAccountFunc))
	v1.GET("/accounts", echo.HandlerFunc(listAccountsFunc))
	v1.GET("/accounts/:id", echo.HandlerFunc(getByIDAccountFunc))
	v1.PUT("/accounts/:id/blocks", echo.HandlerFunc(blockByIDFunc))
	v1.PUT("/accounts/:id/unblocks", echo.HandlerFunc(unblockByIDFunc))
	v1.PUT("/accounts/:id/closes", echo.HandlerFunc(closeByIDFunc))
	v1.GET("/accounts/:id/statements", echo.HandlerFunc(listAccountStatementFunc))
	v1.GET("/accounts/:id/balances", echo.HandlerFunc(getBalanceByIDAccountFunc))
	v1.POST("/transactions/credits", echo.HandlerFunc(createCreditTransactionFunc))
	v1.POST("/transactions/debits", echo.HandlerFunc(createDebitTransactionFunc))
	v1.POST("/transactions/p2p", echo.HandlerFunc(createP2PTransactionFunc))
	v1.GET("/transactions/:id", echo.HandlerFunc(getByIDTransactionFunc))

	hmux := http.NewServeMux()
	hmux.Handle("/", e)
	if env.DebugPprof {
		hmux.HandleFunc("/debug/pprof/", pprof.Index)
		hmux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		hmux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		hmux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		hmux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	apiMiddlewares := make([]middlewares.Middleware, 0, 3)
	apiMiddlewares = append(apiMiddlewares, middlewares.NewTracerHTTPMiddleware(t, "/", "/readiness", "/liveness"))
	apiMiddlewares = append(apiMiddlewares, middlewares.NewRecoveryHTTPMiddleware())
	apiMiddlewares = append(apiMiddlewares, middlewares.NewDefaultContentTypeValidator())

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", env.HTTPPort),
		Handler: middlewares.Chain(hmux, apiMiddlewares...),
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				zap.L().Info(
					"http_server_up",
					zap.String("description", "up and running api server"),
					zap.String("address", httpServer.Addr),
				)
				if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
					zap.L().Error("http_server_down", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return httpServer.Shutdown(ctx)
		},
	})

	return nil
}
