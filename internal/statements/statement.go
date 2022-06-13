package statements

import (
	"time"

	"github.com/dalmarcogd/dock-test/internal/accounts"
)

type Statement struct {
	FromAccount accounts.Account
	ToAccount   accounts.Account
	Amount      float64
	Description string
	CreatedAt   time.Time
}
