package store

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/iurnickita/gophermart/internal/model"
	"github.com/iurnickita/gophermart/internal/store/config"
)

type Store interface {
	BalanceGetActual(ctx context.Context, customer string) (model.Balance, error)
	BalanceGetWithdrawals(ctx context.Context, customer string) ([]model.Balance, error)
	BalanceGetHistory(ctx context.Context, customer string) ([]model.Balance, error)
	BalanceIncrease(ctx context.Context, customer string, order string, points int) error
	BalanceDecrease(ctx context.Context, customer string, order string, points int) error
	PurchaseOrderPost(ctx context.Context, order model.PurchaseOrder) error
	PurchaseOrderPut(ctx context.Context, order model.PurchaseOrder) error
	PurchaseOrderGet(ctx context.Context, customer string) ([]model.PurchaseOrder, error)
}

var (
	ErrAlreadyExists     = errors.New("already exists")
	ErrDuplicateRequest  = errors.New("duplicate request")
	ErrPointsIncorrect   = errors.New("points value is incorrect")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type store struct {
	database     *sql.DB
	balanceMutex map[string]*sync.Mutex
}

func NewStore(cfg config.Config) (Store, error) {
	db, err := sql.Open("pgx", cfg.DBDsn)
	if err != nil {
		return nil, err
	}

	// Таблица баланса пользователя
	// Представлят собой журнал. Для каждой новой операции пользователя создается новая запись,
	// так легче отслеживать историю и выявлять ошибки при операциях с балансом
	// [не реализовано] Блокировка на уровне пользователя *костыль: store.balanceMutex[customer]mutex
	// [не реализовано] Записи нельзя редактировать/удалять
	_, err = db.Exec(
		"CREATE TABLE IF NOT EXISTS balance (" +
			" customer VARCHAR (10) PRIMARY KEY," +
			" operation SERIAL PRIMARY KEY," +
			" timestamp TIMESTAMP NOT NULL," +
			" difference INTEGER NOT NULL," +
			" balance INTEGER," +
			" withdrawn INTEGER," +
			" order VARCHAR (10) NOT NULL" +
			" );")
	if err != nil {
		return nil, err
	}

	// Таблица заказов.
	// Создается одна строка на заказ, после чего меняется ее статус
	_, err = db.Exec(
		"CREATE TABLE IF NOT EXISTS purchase_order (" +
			" number VARCHAR (10) PRIMARY KEY," +
			" customer VARCHAR (10) NOT NULL," +
			" status VARCHAR (10) NOT NULL," +
			" accrual INTEGER NOT NULL," +
			" uploaded_at TIMESTAMP NOT NULL," +
			" );")
	if err != nil {
		return nil, err
	}

	return &store{
		database: db,
	}, nil
}

func (store *store) BalanceGetActual(ctx context.Context, customer string) (model.Balance, error) {
	//Получение актуального баланса
	var balanceRow model.Balance
	row := store.database.QueryRowContext(ctx,
		"SELECT customer, operation, timestamp, difference, balance, withdrawn, order"+
			" FROM balance ORDER BY operation"+
			" WHERE customer = $1"+
			" LIMIT 1",
		customer)
	err := row.Scan(&balanceRow.Key.Customer,
		&balanceRow.Key.Operation,
		&balanceRow.Data.Timestamp,
		&balanceRow.Data.Difference,
		&balanceRow.Data.Balance,
		&balanceRow.Data.Withdrawn,
		&balanceRow.Data.Order)
	if err != nil && err != sql.ErrNoRows { // если нет строки - ок
		return model.Balance{}, err
	}
	return balanceRow, nil
}

func (store *store) BalanceGetWithdrawals(ctx context.Context, customer string) ([]model.Balance, error) {
	//Получение списаний
	rows, err := store.database.QueryContext(ctx,
		"SELECT customer, operation, timestamp, difference, balance, withdrawn, order"+
			" FROM balance ORDER BY operation"+
			" WHERE customer = $1"+
			"   AND difference < 0"+
			" LIMIT 1",
		customer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var withdrawals []model.Balance
	for rows.Next() {
		var balanceRow model.Balance
		err := rows.Scan(&balanceRow.Key.Customer,
			&balanceRow.Key.Operation,
			&balanceRow.Data.Timestamp,
			&balanceRow.Data.Difference,
			&balanceRow.Data.Balance,
			&balanceRow.Data.Withdrawn,
			&balanceRow.Data.Order)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, balanceRow)
	}

	return withdrawals, nil
}

func (store *store) BalanceGetHistory(ctx context.Context, customer string) ([]model.Balance, error) {
	return nil, nil
}

func (store store) BalanceIncrease(ctx context.Context, customer string, order string, points int) error {
	//Блокировка баланса пользователя
	mutex := store.balanceMutex[customer]
	mutex.Lock()
	defer mutex.Unlock()

	if points <= 0 {
		return ErrPointsIncorrect
	}

	//Получение актуального баланса
	balanceRow, err := store.BalanceGetActual(ctx, customer)
	if err != nil {
		return err
	}

	//Запись обновленного баланса
	balanceRow.Key.Customer = customer
	balanceRow.Data.Timestamp = time.Now()
	balanceRow.Data.Difference = points
	balanceRow.Data.Balance += points
	//balanceRow.Data.Withdrawn
	balanceRow.Data.Order = order
	_, err = store.database.ExecContext(ctx,
		"INSERT INTO balance (customer, timestamp, difference, balance, withdrawn, order)"+
			" VALUES (&1, &2, &3, &4, &5, &6)",
		balanceRow.Key.Customer,
		balanceRow.Data.Timestamp,
		balanceRow.Data.Difference,
		balanceRow.Data.Balance,
		balanceRow.Data.Withdrawn,
		balanceRow.Data.Order)
	if err != nil {
		return err
	}

	return nil
}

func (store *store) BalanceDecrease(ctx context.Context, customer string, order string, points int) error {
	//Блокировка баланса пользователя
	mutex := store.balanceMutex[customer]
	mutex.Lock()
	defer mutex.Unlock()

	if points <= 0 {
		return ErrPointsIncorrect
	}

	//Получение актуального баланса
	var balanceRow model.Balance
	row := store.database.QueryRowContext(ctx,
		"SELECT customer, operation, timestamp, difference, balance, withdrawn, order"+
			" FROM balance ORDER BY operation"+
			" WHERE customer = $1"+
			" LIMIT 1",
		customer)
	err := row.Scan(&balanceRow.Key.Customer,
		&balanceRow.Key.Operation,
		&balanceRow.Data.Timestamp,
		&balanceRow.Data.Difference,
		&balanceRow.Data.Balance,
		&balanceRow.Data.Withdrawn,
		&balanceRow.Data.Order)
	if err != nil {
		return err
	}

	//Проверка достаточно средств
	if balanceRow.Data.Balance < points {
		return ErrInsufficientFunds
	}

	//Запись обновленного баланса
	balanceRow.Key.Customer = customer
	balanceRow.Data.Timestamp = time.Now()
	balanceRow.Data.Difference = -points
	balanceRow.Data.Balance -= points
	balanceRow.Data.Withdrawn += points
	balanceRow.Data.Order = order
	_, err = store.database.ExecContext(ctx,
		"INSERT INTO balance (customer, timestamp, difference, balance, withdrawn, order)"+
			" VALUES (&1, &2, &3, &4, &5, &6)",
		balanceRow.Key.Customer,
		balanceRow.Data.Timestamp,
		balanceRow.Data.Difference,
		balanceRow.Data.Balance,
		balanceRow.Data.Withdrawn,
		balanceRow.Data.Order)
	if err != nil {
		return err
	}

	return nil
}

func (store *store) PurchaseOrderPost(ctx context.Context, order model.PurchaseOrder) error {
	//Запись нового заказа
	_, err := store.database.ExecContext(ctx,
		"INSERT INTO purchase_order (number, customer, status, accrual, uploaded_at)"+
			" VALUES (&1, &2, &3, &4, &5)",
		order.Number,
		order.Data.Customer,
		order.Data.Status,
		order.Data.Accrual,
		order.Data.UploadedAt)
	if err != nil {
		row := store.database.QueryRowContext(ctx,
			"SELECT customer FROM purchase_order"+
				" WHERE number = $1",
			order.Number)
		var customer string
		err = row.Scan(&customer)
		if err == nil {
			if customer != order.Data.Customer {
				return ErrDuplicateRequest
			}
		}
		return ErrAlreadyExists
	}
	return nil
}

func (store *store) PurchaseOrderPut(ctx context.Context, order model.PurchaseOrder) error {
	//Обновление статуса заказа
	_, err := store.database.ExecContext(ctx,
		"UPDATE purchase_order AS o"+
			" SET status = $1"+
			" WHERE number = $2"+
			"   AND customer = $3",
		order.Data.Status,
		order.Number,
		order.Data.Customer)
	if err != nil {
		return err
	}
	return nil
}

func (store *store) PurchaseOrderGet(ctx context.Context, customer string) ([]model.PurchaseOrder, error) {
	//Получение заказов
	rows, err := store.database.QueryContext(ctx,
		"SELECT number, customer, status, accrual, uploaded_at"+
			" FROM purchase_order"+
			" WHERE customer = $1",
		customer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []model.PurchaseOrder
	for rows.Next() {
		var orderRow model.PurchaseOrder
		err := rows.Scan(&orderRow.Number,
			&orderRow.Data.Customer,
			&orderRow.Data.Status,
			&orderRow.Data.Accrual,
			&orderRow.Data.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, orderRow)
	}

	return orders, nil
}
