package store

import (
	"context"
	"testing"

	"github.com/iurnickita/gophermart/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStoreBalance(t *testing.T) {
	const (
		customer = "100001"
		order    = "100001"
		points   = 300
	)

	cfg := config.GetConfig()

	store, err := NewStore(cfg.Store)
	if err != nil {
		require.NoError(t, err)
	}

	// начальный баланс
	balance, err := store.BalanceGetActual(context.Background(), customer)
	if err != nil {
		require.NoError(t, err)
	}
	startBalance := balance.Data.Balance

	// увеличение на 300
	err = store.BalanceIncrease(context.Background(), customer, order, points)
	if err != nil {
		require.NoError(t, err)
	}

	// конечный баланс
	balance, err = store.BalanceGetActual(context.Background(), customer)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, balance.Data.Balance, startBalance+points)

	// уменьшение на 300
	err = store.BalanceDecrease(context.Background(), customer, order, points)
	if err != nil {
		require.NoError(t, err)
	}

	// конечный баланс
	balance, err = store.BalanceGetActual(context.Background(), customer)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, balance.Data.Balance, startBalance)
}
