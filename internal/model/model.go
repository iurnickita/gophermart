package model

import "time"

// Входящие заказы

type PurchaseOrder struct {
	Number string
	Data   PurchaseOrderData
}
type PurchaseOrderData struct {
	Customer   string
	Status     string
	Accrual    int
	UploadedAt time.Time
}

const (
	PurchaseOrderStatusNew        = "NEW"
	PurchaseOrderStatusProcessing = "PROCESSING"
	PurchaseOrderStatusInvalid    = "INVALID"
	PurchaseOrderStatusProcessed  = "PROCESSED"
)

// Баланс и история

type Balance struct {
	Key  BalanceKey
	Data BalanceData
}
type BalanceKey struct {
	Customer  string
	Operation string
}
type BalanceData struct {
	Timestamp  time.Time
	Difference int
	Balance    int
	Withdrawn  int
	Order      string
}
