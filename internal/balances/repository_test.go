//go:build integration

package balances

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dalmarcogd/dock-test/pkg/database"
	"github.com/dalmarcogd/dock-test/pkg/testingcontainers"
	"github.com/dalmarcogd/dock-test/pkg/tracer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	url, closeFunc, err := testingcontainers.NewPostgresContainer()
	assert.NoError(t, err)
	defer closeFunc(ctx) //nolint:errcheck

	_, callerPath, _, _ := runtime.Caller(0) //nolint:dogsled
	err = testingcontainers.RunMigrateDatabase(
		url,
		fmt.Sprintf("file://%s/../../migrations/", filepath.Dir(callerPath)),
	)
	assert.NoError(t, err)

	db, err := database.New(tracer.NewNoop(), url, url)
	assert.NoError(t, err)

	repo := NewRepository(tracer.NewNoop(), db)

	db.Master().ExecContext(ctx, "select *")

	repo.GetByAccountID()
}
