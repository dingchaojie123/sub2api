package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLotteryRepositoryBindPrizeCodesUsesUpdatedPrizeValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	mock.ExpectExec(regexp.QuoteMeta("AND value IN (30, 10, 5, 2)")).
		WithArgs(sqlmock.AnyArg(), service.RedeemTypeBalance, service.StatusUnused).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &lotteryRepository{db: db}
	bound, err := repo.BindPrizeCodes(context.Background(), []int64{11})
	require.NoError(t, err)
	require.EqualValues(t, 1, bound)
	require.NoError(t, mock.ExpectationsWereMet())
}
