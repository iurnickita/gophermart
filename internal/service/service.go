package service

import (
	"context"
	"errors"
	"time"

	"github.com/iurnickita/gophermart/internal/balance"
	"github.com/iurnickita/gophermart/internal/model"
	"github.com/iurnickita/gophermart/internal/service/accrualclient"
	"github.com/iurnickita/gophermart/internal/service/config"
	"github.com/iurnickita/gophermart/internal/store"
)

type Service interface {
	PostOrder(order model.PurchaseOrder) error
	GetOrder(customer string) ([]model.PurchaseOrder, error)
	GetBalance(customer string) (model.Balance, error)
	PostWithdraw(order model.PurchaseOrder, points int) error
	GetWithdrawals(customer string) ([]model.Balance, error)
}

var (
	ErrInsufficientData    = errors.New("insufficient data")
	ErrUnprocessableEntity = errors.New("unprocessable entity")
	ErrAlreadyExists       = errors.New("already exists")
	ErrDuplicateRequest    = errors.New("duplicate request")
	ErrInsufficientFunds   = errors.New("insufficient funds")
)

type service struct {
	cfg     config.Config
	store   store.Store
	balance balance.Balance
	accrual accrualclient.AccrualClient
}

func NewService(cfg config.Config, store store.Store) (Service, error) {
	balance := balance.NewBalance(store)
	accrual := accrualclient.NewAccrualClient(cfg.AccrualAddr)

	service := service{
		cfg:     cfg,
		store:   store,
		balance: balance,
		accrual: accrual}

	return &service, nil
}

func (service *service) PostOrder(order model.PurchaseOrder) error {
	ctx := context.Background()

	if order.Number == "" {
		return ErrInsufficientData
	}
	if order.Data.Customer == "" {
		return ErrInsufficientData
	}
	// Проверка по алгоритму Луна
	// ... ErrUnprocessableEntity

	var newOrder model.PurchaseOrder
	newOrder.Number = order.Number
	newOrder.Data.Customer = order.Data.Customer
	newOrder.Data.Status = model.PurchaseOrderStatusNew
	newOrder.Data.UploadedAt = time.Now()

	err := service.store.PurchaseOrderPost(ctx, newOrder)
	if err != nil {
		switch err {
		case store.ErrAlreadyExists:
			return ErrAlreadyExists
		case store.ErrDuplicateRequest:
			return ErrDuplicateRequest
		default:
			return err
		}
	}

	go service.accrualProcessing(newOrder)

	return nil
}

func (service *service) accrualProcessing(order model.PurchaseOrder) {
	ctx := context.Background()

	var accrualAnswer accrualclient.AccrualAnswer
	var err error

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			accrualAnswer, err = service.accrual.GetAccrual(order)
			if err != nil {
				// retry бы тут
				return
			}
			switch accrualAnswer.Status {
			case accrualclient.AccrualStatusProcessing:
				if order.Data.Status != accrualclient.AccrualStatusProcessing {
					order.Data.Status = model.PurchaseOrderStatusInvalid
					service.store.PurchaseOrderPut(ctx, order)
				}
			case accrualclient.AccrualStatusInvalid:
				order.Data.Status = model.PurchaseOrderStatusInvalid
				service.store.PurchaseOrderPut(ctx, order)
				return
			case accrualclient.AccrualStatusProcessed:
				order.Data.Status = model.PurchaseOrderStatusProcessed
				order.Data.Accrual = accrualAnswer.Accrual
				service.store.PurchaseOrderPut(ctx, order)
				service.balance.Increase(order.Data.Customer, order.Number, accrualAnswer.Accrual)
				return
			default:
			}
		}
	}
}

func (service *service) GetOrder(customer string) ([]model.PurchaseOrder, error) {
	ctx := context.Background()

	if customer == "" {
		return nil, ErrInsufficientData
	}

	return service.store.PurchaseOrderGet(ctx, customer)
}

func (service *service) GetBalance(customer string) (model.Balance, error) {
	if customer == "" {
		return model.Balance{}, ErrInsufficientData
	}

	return service.balance.Get(customer)
}

func (service *service) PostWithdraw(order model.PurchaseOrder, points int) error {
	if order.Number == "" {
		return ErrInsufficientData
	}
	if order.Data.Customer == "" {
		return ErrInsufficientData
	}
	if points == 0 {
		return ErrInsufficientData
	}
	// Проверка по алгоритму Луна
	// ... ErrUnprocessableEntity

	err := service.balance.Decrease(order.Data.Customer, order.Number, points)
	if err != nil {
		switch err {
		case store.ErrInsufficientFunds:
			return ErrInsufficientFunds
		default:
			return err
		}
	}
	return nil
}

func (service *service) GetWithdrawals(customer string) ([]model.Balance, error) {
	if customer == "" {
		return nil, ErrInsufficientData
	}

	return service.balance.GetWithdrawals(customer)
}
