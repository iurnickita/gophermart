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
		Handler: gzip.GzipMiddleware(logger.RequestLogMdlw(router, h.zaplog)),
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
	mux.HandleFunc("POST /api/user/register", h.auth.Register)
	mux.HandleFunc("POST /api/user/login", h.auth.Login)
	mux.HandleFunc("POST /api/user/orders", h.auth.Middleware(h.PostOrder))
	mux.HandleFunc("GET /api/user/orders", h.auth.Middleware(h.GetOrder))
	mux.HandleFunc("GET /api/user/balance", h.auth.Middleware(h.GetBalance))
	mux.HandleFunc("POST /api/user/balance/withdraw", h.auth.Middleware(h.PostWithdraw))
	mux.HandleFunc("GET /api/user/withdrawals", h.auth.Middleware(h.GetWithdrawals))

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
	err = h.service.PostOrder(r.Context(), order)
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
	Number     string  `json:"number"`
	Status     string  `json:"status"`
	Accrual    float32 `json:"accrual"`
	UploadedAt string  `json:"uploaded_at"`
}

func (h *handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	orders, err := h.service.GetOrder(r.Context(), userCode)
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
				Status:     order.Data.Status,
				Accrual:    h.pointsOutput(order.Data.Accrual),
				UploadedAt: order.Data.UploadedAt.Format(time.RFC3339)})
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
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}

func (h *handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	balance, err := h.service.GetBalance(r.Context(), userCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balanceJSON := GetBalanceJSONResponse{Current: h.pointsOutput(balance.Data.Balance),
		Withdrawn: h.pointsOutput(balance.Data.Withdrawn)}
	responseJSON, err := json.Marshal(balanceJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

type PostWithdrawJSONRequest struct {
	Order string  `json:"order"`
	Sum   float32 `json:"sum"`
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
	err = h.service.PostWithdraw(r.Context(), order, h.pointsInput(withdrawJSON.Sum))
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
	Order       string  `json:"order"`
	Sum         float32 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func (h *handler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userCode := r.Header.Get(auth.HeaderUserCodeKey)

	withdrawals, err := h.service.GetWithdrawals(r.Context(), userCode)
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
				Sum:         h.pointsOutput(-withdraw.Data.Difference),
				ProcessedAt: withdraw.Data.Timestamp.Format(time.RFC3339)})
	}
	responseJSON, err := json.Marshal(withdrawalsJSON)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

func (h *handler) pointsOutput(points int) float32 {
	return float32(points) / 100
}

func (h *handler) pointsInput(points float32) int {
	return int(points * 100)
}
