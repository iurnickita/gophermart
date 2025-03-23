package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/iurnickita/gophermart/internal/model"
	"github.com/iurnickita/gophermart/internal/service/accrualclient"
	"github.com/iurnickita/gophermart/internal/service/config"
	"github.com/iurnickita/gophermart/internal/store"
	"github.com/theplant/luhn"
	"go.uber.org/zap"
)

type Service interface {
	PostOrder(ctx context.Context, order model.PurchaseOrder) error
	GetOrder(ctx context.Context, customer string) ([]model.PurchaseOrder, error)
	GetBalance(ctx context.Context, customer string) (model.Balance, error)
	PostWithdraw(ctx context.Context, order model.PurchaseOrder, points int) error
	GetWithdrawals(ctx context.Context, customer string) ([]model.Balance, error)
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
	zaplog  *zap.Logger
	accrual serviceAccrual
}

type serviceAccrual struct {
	client  accrualclient.AccrualClient
	orders  chan model.PurchaseOrder
	timeout time.Time
}

func NewService(cfg config.Config, store store.Store, zaplog *zap.Logger) (Service, error) {
	service := service{
		cfg:    cfg,
		store:  store,
		zaplog: zaplog}

	service.accrualRunWorkerPool()

	return &service, nil
}

func (service *service) accrualRunWorkerPool() {
	// Клиент
	accrualClient := accrualclient.NewAccrualClient(service.cfg.AccrualAddr)
	service.accrual.client = accrualClient

	// Число воркеров
	numWorkers := service.cfg.AccrualWorkers
	if numWorkers < 1 {
		numWorkers = 5
	}

	// Каналы
	service.accrual.orders = make(chan model.PurchaseOrder, numWorkers)
	results := make(chan accrualResult, numWorkers)

	// Создание воркеров
	for w := 1; w <= numWorkers; w++ {
		go func(orders <-chan model.PurchaseOrder, results chan<- accrualResult) {
			for order := range orders {
				results <- service.accrualProcessing(order)
			}
		}(service.accrual.orders, results)
	}

	// Retry
	retryInterval := time.Duration(service.cfg.AccrualRetry) * time.Second
	if retryInterval == 0 {
		retryInterval = 5 * time.Second
	}
	for r := range results {
		if r.err != nil {
			service.accrualAddForProcessing(r.order, retryInterval)
		}
	}
}

func (service *service) accrualAddForProcessing(order model.PurchaseOrder, wait time.Duration) {
	go func() {
		time.Sleep(wait)
		service.accrual.orders <- order
	}()
}

type accrualResult struct {
	order model.PurchaseOrder
	err   error
}

func (service *service) accrualProcessing(order model.PurchaseOrder) accrualResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var accrualAnswer accrualclient.AccrualAnswer
	var retryAfter int
	var err error

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			return accrualResult{order: order, err: ctx.Err()}
		case <-ticker.C:
			if service.accrual.timeout.Before(time.Now()) { // проверка общего таймаута
				// Запрос к accral
				accrualAnswer, retryAfter, err = service.accrual.client.GetAccrual(order)
				if err != nil {
					service.zaplog.Info("accrual error",
						zap.String("error:", err.Error()),
					)
					if err == accrualclient.ErrTooManyRequests { // установка общего таймаута
						service.accrual.timeout = time.Now().Add(time.Duration(retryAfter) * time.Second)
					}
					return accrualResult{order: order, err: err}
				}
				service.zaplog.Info("accrual answer",
					zap.String("status:", accrualAnswer.Status),
				)
				// Обработка ответа
				switch accrualAnswer.Status {
				case accrualclient.AccrualStatusProcessing:
					if order.Data.Status != model.PurchaseOrderStatusProcessing {
						order.Data.Status = model.PurchaseOrderStatusProcessing
						service.store.PurchaseOrderPut(ctx, order)
					}
				case accrualclient.AccrualStatusInvalid:
					order.Data.Status = model.PurchaseOrderStatusInvalid
					service.store.PurchaseOrderPut(ctx, order)
					return accrualResult{order: order, err: nil}
				case accrualclient.AccrualStatusProcessed:
					order.Data.Status = model.PurchaseOrderStatusProcessed
					order.Data.Accrual = accrualAnswer.Accrual
					service.store.PurchaseOrderPut(ctx, order)
					service.store.BalanceIncrease(ctx, order.Data.Customer, order.Number, accrualAnswer.Accrual)
					return accrualResult{order: order, err: nil}
				default:
				}
			}
		}
	}
}

func (service *service) PostOrder(ctx context.Context, order model.PurchaseOrder) error {
	if order.Number == "" {
		return ErrInsufficientData
	}
	if order.Data.Customer == "" {
		return ErrInsufficientData
	}
	// Проверка по алгоритму Луна
	numberInt, err := strconv.Atoi(order.Number)
	if err != nil {
		return ErrUnprocessableEntity
	}
	if !luhn.Valid(numberInt) {
		return ErrUnprocessableEntity
	}

	var newOrder model.PurchaseOrder
	newOrder.Number = order.Number
	newOrder.Data.Customer = order.Data.Customer
	newOrder.Data.Status = model.PurchaseOrderStatusNew
	newOrder.Data.UploadedAt = time.Now()

	err = service.store.PurchaseOrderPost(ctx, newOrder)
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

	service.accrualAddForProcessing(newOrder, 0)

	return nil
}

func (service *service) GetOrder(ctx context.Context, customer string) ([]model.PurchaseOrder, error) {
	if customer == "" {
		return nil, ErrInsufficientData
	}

	return service.store.PurchaseOrderGet(ctx, customer)
}

func (service *service) GetBalance(ctx context.Context, customer string) (model.Balance, error) {
	if customer == "" {
		return model.Balance{}, ErrInsufficientData
	}

	return service.store.BalanceGetActual(ctx, customer)
}

func (service *service) PostWithdraw(ctx context.Context, order model.PurchaseOrder, points int) error {
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
	numberInt, err := strconv.Atoi(order.Number)
	if err != nil {
		return ErrUnprocessableEntity
	}
	if !luhn.Valid(numberInt) {
		return ErrUnprocessableEntity
	}

	err = service.store.BalanceDecrease(ctx, order.Data.Customer, order.Number, points)
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

func (service *service) GetWithdrawals(ctx context.Context, customer string) ([]model.Balance, error) {
	if customer == "" {
		return nil, ErrInsufficientData
	}

	return service.store.BalanceGetWithdrawals(ctx, customer)
}
