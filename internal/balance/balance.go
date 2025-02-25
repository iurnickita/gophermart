package balance

import (
	"context"

	"github.com/iurnickita/gophermart/internal/model"
	"github.com/iurnickita/gophermart/internal/store"
)

type Balance interface {
	Increase(customer string, order string, points int) error
	Decrease(customer string, order string, points int) error
	Get(customer string) (model.Balance, error)
	GetWithdrawals(customer string) ([]model.Balance, error)
	GetHistory(customer string) ([]model.Balance, error)
}

type balance struct {
	store store.Store
}

func NewBalance(store store.Store) Balance {
	balance := balance{store: store}
	return &balance
}

func (balance *balance) Get(customer string) (model.Balance, error) {
	ctx := context.Background()

	return balance.store.BalanceGetActual(ctx, customer)
}

func (balance *balance) GetWithdrawals(customer string) ([]model.Balance, error) {
	ctx := context.Background()

	return balance.store.BalanceGetWithdrawals(ctx, customer)
}

func (balance *balance) GetHistory(customer string) ([]model.Balance, error) {
	ctx := context.Background()

	return balance.store.BalanceGetHistory(ctx, customer)
}

func (balance *balance) Increase(customer string, order string, points int) error {
	ctx := context.Background()

	return balance.store.BalanceIncrease(ctx, customer, order, points)
}

func (balance *balance) Decrease(customer string, order string, points int) error {
	ctx := context.Background()

	return balance.store.BalanceDecrease(ctx, customer, order, points)
}
