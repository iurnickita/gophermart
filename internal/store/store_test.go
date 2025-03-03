package store

import (
	"context"
	"testing"
	"time"

	"github.com/iurnickita/gophermart/internal/config"
	"github.com/iurnickita/gophermart/internal/model"
	"github.com/stretchr/testify/require"
)

func TestStoreBalance(t *testing.T) {
	const (
		customer = "100001"
		order    = "100001"
		points   = 300
	)

	cfg := config.GetConfig()
	ctx := context.Background()

	store, err := NewStore(cfg.Store)
	if err != nil {
		require.NoError(t, err)
	}

	// начальный баланс
	balance, err := store.BalanceGetActual(ctx, customer)
	if err != nil {
		require.NoError(t, err)
	}
	startBalance := balance.Data.Balance

	// увеличение на 300
	err = store.BalanceIncrease(ctx, customer, order, points)
	if err != nil {
		require.NoError(t, err)
	}

	// конечный баланс
	balance, err = store.BalanceGetActual(ctx, customer)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, balance.Data.Balance, startBalance+points)

	// уменьшение на 300
	err = store.BalanceDecrease(ctx, customer, order, points)
	if err != nil {
		require.NoError(t, err)
	}

	// конечный баланс
	balance, err = store.BalanceGetActual(ctx, customer)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, balance.Data.Balance, startBalance)
}

func TestStorePurchaseOrder(t *testing.T) {
	const (
		customer = "100001"
		number   = "100001"
	)

	cfg := config.GetConfig()
	ctx := context.Background()

	store, err := NewStore(cfg.Store)
	if err != nil {
		require.NoError(t, err)
	}

	// Создание заказа
	var order model.PurchaseOrder
	order.Number = number
	order.Data.Customer = customer
	order.Data.Status = model.PurchaseOrderStatusNew
	order.Data.UploadedAt = time.Now().UTC()
	err = store.PurchaseOrderPost(ctx, order)
	if err != nil && err != ErrDuplicateRequest {
		require.NoError(t, err)
	}

	// Чтение заказа
	dbOrders, err := store.PurchaseOrderGet(ctx, customer)
	if err != nil {
		require.NoError(t, err)
	}
	for _, dbOrder := range dbOrders {
		if dbOrder.Number == number {
			order.Data.UploadedAt = time.Time{}
			dbOrder.Data.UploadedAt = time.Time{}

			require.Equal(t, dbOrder, order)
			break
		}
	}

	// Обновление заказа
	order.Data.Status = model.PurchaseOrderStatusProcessed
	order.Data.Accrual = 500
	err = store.PurchaseOrderPut(ctx, order)
	if err != nil {
		require.NoError(t, err)
	}

	// Чтение заказа
	dbOrders, err = store.PurchaseOrderGet(ctx, customer)
	if err != nil {
		require.NoError(t, err)
	}
	for _, dbOrder := range dbOrders {
		if dbOrder.Number == number {
			order.Data.UploadedAt = time.Time{}
			dbOrder.Data.UploadedAt = time.Time{}

			require.Equal(t, dbOrder, order)
			break
		}
	}
}

func TestStoreAuth(t *testing.T) {
	const (
		login    = "100001"
		password = "100001"
	)

	cfg := config.GetConfig()
	ctx := context.Background()

	store, err := NewStore(cfg.Store)
	if err != nil && err != ErrDuplicateRequest {
		require.NoError(t, err)
	}

	userCodeRegister, err := store.AuthRegister(ctx, login, password)
	if err != nil {
		require.NoError(t, err)
	}

	userCodeLogin, err := store.AuthLogin(ctx, login, password)
	if err != nil {
		require.NoError(t, err)
	}

	require.Equal(t, userCodeRegister, userCodeLogin)
}
