package accrualclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/iurnickita/gophermart/internal/model"
)

// JSON ответ accrual
type AccrualAnswer struct {
	Order   string `json:"order"`
	Status  string `json:"status"`
	Accrual int    `json:"accrual"`
}

const (
	AccrualStatusRegistered = "REGISTERED"
	AccrualStatusInvalid    = "INVALID"
	AccrualStatusProcessing = "PROCESSING"
	AccrualStatusProcessed  = "PROCESSED"
)

type AccrualClient interface {
	GetAccrual(order model.PurchaseOrder) (AccrualAnswer, error)
}

type accrualClient struct {
	serviceAddr string
}

func NewAccrualClient(serviceAddr string) AccrualClient {
	return accrualClient{serviceAddr: serviceAddr}
}

func (client accrualClient) GetAccrual(order model.PurchaseOrder) (AccrualAnswer, error) {
	path := "/api/orders/"

	setreq := resty.New().R()
	setreq.Method = http.MethodGet
	setreq.URL = client.serviceAddr + path + order.Number
	setresp, err := setreq.Send()
	if err != nil {
		return AccrualAnswer{}, err
	}

	switch setresp.StatusCode() {
	case http.StatusOK:
		var accrualAnswer AccrualAnswer
		err = json.Unmarshal(setresp.Body(), &accrualAnswer)
		return accrualAnswer, err
	default:
		return AccrualAnswer{}, fmt.Errorf("accrual request status: %d", setresp.StatusCode())
	}
}
