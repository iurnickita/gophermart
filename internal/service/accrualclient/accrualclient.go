package accrualclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/iurnickita/gophermart/internal/model"
)

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
	GetAccrual(order model.PurchaseOrder) (AccrualAnswer, int, error)
}

var (
	ErrTooManyRequests = errors.New("429 Too Many Requests")
)

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

func (client accrualClient) GetAccrual(order model.PurchaseOrder) (AccrualAnswer, int, error) {
	path := "/api/orders/"

	setreq := resty.New().R()
	setreq.Method = http.MethodGet
	setreq.URL = client.serviceAddr + path + order.Number
	setresp, err := setreq.Send()
	if err != nil {
		return AccrualAnswer{}, 0, err
	}

	switch setresp.StatusCode() {
	case http.StatusOK:
		var accrualAnswerJSON GetAccrualAnswerJSON
		err = json.Unmarshal(setresp.Body(), &accrualAnswerJSON)
		if err != nil {
			return AccrualAnswer{}, 0, err
		}
		var accrualAnswer AccrualAnswer
		accrualAnswer.Order = accrualAnswerJSON.Order
		accrualAnswer.Status = accrualAnswerJSON.Status
		accrualAnswer.Accrual = int(accrualAnswerJSON.Accrual * 100)
		return accrualAnswer, 0, err
	case http.StatusTooManyRequests:
		retryAfter, err := strconv.Atoi(setresp.Header().Get("Retry-After"))
		if err != nil {
			retryAfter = 0
		}
		return AccrualAnswer{}, retryAfter, ErrTooManyRequests
	default:
		return AccrualAnswer{}, 0, fmt.Errorf("accrual request status: %d", setresp.StatusCode())
	}
}
