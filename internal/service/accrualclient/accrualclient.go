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
	Order   string
	Status  string
	Accrual int
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

type GetAccrualAnswerJSON struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float32 `json:"accrual"`
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
		var accrualAnswerJSON GetAccrualAnswerJSON
		err = json.Unmarshal(setresp.Body(), &accrualAnswerJSON)
		if err != nil {
			return AccrualAnswer{}, err
		}
		var accrualAnswer AccrualAnswer
		accrualAnswer.Order = accrualAnswerJSON.Order
		accrualAnswer.Status = accrualAnswerJSON.Status
		accrualAnswer.Accrual = int(accrualAnswerJSON.Accrual * 100)
		return accrualAnswer, err
	default:
		return AccrualAnswer{}, fmt.Errorf("accrual request status: %d", setresp.StatusCode())
	}
}
