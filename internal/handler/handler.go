package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/iurnickita/gophermart/internal/auth"
	"github.com/iurnickita/gophermart/internal/gzip"
	"github.com/iurnickita/gophermart/internal/handler/config"
	"github.com/iurnickita/gophermart/internal/logger"
	"github.com/iurnickita/gophermart/internal/model"
	"github.com/iurnickita/gophermart/internal/service"
	"go.uber.org/zap"
)

func Serve(cfg config.Config, auth auth.Auth, service service.Service, zaplog *zap.Logger) error {
	h := newHandler(auth, service, cfg.ServerAddr, zaplog)
	router := h.newRouter()

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: router,
	}

	return srv.ListenAndServe()
}

type handler struct {
	auth     auth.Auth
	service  service.Service
	baseaddr string
	zaplog   *zap.Logger
}

func newHandler(auth auth.Auth, service service.Service, baseaddr string, zaplog *zap.Logger) *handler {
	return &handler{
		auth:     auth,
		service:  service,
		baseaddr: baseaddr,
		zaplog:   zaplog,
	}
}

func (h *handler) newRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/user/register", gzip.GzipMiddleware(logger.RequestLogMdlw(h.auth.Register, h.zaplog)))
	mux.HandleFunc("POST /api/user/login", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Login), h.zaplog))
	mux.HandleFunc("POST /api/user/orders", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Middleware(h.PostOrder)), h.zaplog))
	mux.HandleFunc("GET /api/user/orders", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Middleware(h.GetOrder)), h.zaplog))
	mux.HandleFunc("GET /api/user/balance", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Middleware(h.GetBalance)), h.zaplog))
	mux.HandleFunc("POST /api/user/balance/withdraw", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Middleware(h.PostWithdraw)), h.zaplog))
	mux.HandleFunc("GET /api/user/withdrawals", logger.RequestLogMdlw(gzip.GzipMiddleware(h.auth.Middleware(h.GetWithdrawals)), h.zaplog))

	return mux
}

func (h *handler) PostOrder(w http.ResponseWriter, r *http.Request) {
	number, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	order := model.PurchaseOrder{Number: string(number),
		Data: model.PurchaseOrderData{Customer: userCode}}
	err = h.service.PostOrder(order)
	if err != nil {
		switch err {
		case service.ErrInsufficientData:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case service.ErrUnprocessableEntity:
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		case service.ErrAlreadyExists:
			http.Error(w, err.Error(), http.StatusConflict)
		case service.ErrDuplicateRequest:
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type GetOrderJSONResponse struct {
	Number      string    `json:"number"`
	Status      string    `json:"status"`
	Accrual     int       `json:"accrual"`
	Uploaded_at time.Time `json:"uploaded_at"`
}

func (h *handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	orders, err := h.service.GetOrder(userCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		http.Error(w, "", http.StatusNoContent)
		return
	}

	var ordersJSON []GetOrderJSONResponse
	for _, order := range orders {
		ordersJSON = append(ordersJSON,
			GetOrderJSONResponse{Number: order.Number,
				Status:      order.Data.Status,
				Accrual:     order.Data.Accrual,
				Uploaded_at: order.Data.UploadedAt})
	}
	responseJSON, err := json.Marshal(ordersJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

type GetBalanceJSONResponse struct {
	Current   int `json:"current"`
	Withdrawn int `json:"withdrawn"`
}

func (h *handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	balance, err := h.service.GetBalance(userCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balanceJSON := GetBalanceJSONResponse{Current: balance.Data.Balance,
		Withdrawn: balance.Data.Withdrawn}
	responseJSON, err := json.Marshal(balanceJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

type PostWithdrawJSONRequest struct {
	Order string `json:"order"`
	Sum   int    `json:"sum"`
}

func (h *handler) PostWithdraw(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var withdrawJSON PostWithdrawJSONRequest
	err = json.Unmarshal(buf.Bytes(), &withdrawJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	order := model.PurchaseOrder{
		Number: withdrawJSON.Order,
		Data:   model.PurchaseOrderData{Customer: userCode}}
	err = h.service.PostWithdraw(order, withdrawJSON.Sum)
	if err != nil {
		switch err {
		case service.ErrInsufficientFunds:
			http.Error(w, err.Error(), http.StatusPaymentRequired)
		case service.ErrUnprocessableEntity:
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
}

type GetWithdrawalsJSONResponse struct {
	Order        string    `json:"order"`
	Sum          int       `json:"sum"`
	Processed_at time.Time `json:"processed_at"`
}

func (h *handler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	withdrawals, err := h.service.GetWithdrawals(userCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		http.Error(w, "", http.StatusNoContent)
		return
	}

	var withdrawalsJSON []GetWithdrawalsJSONResponse
	for _, withdraw := range withdrawals {
		withdrawalsJSON = append(withdrawalsJSON,
			GetWithdrawalsJSONResponse{Order: withdraw.Data.Order,
				Sum:          -withdraw.Data.Difference,
				Processed_at: withdraw.Data.Timestamp})
	}
	responseJSON, err := json.Marshal(withdrawalsJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}
