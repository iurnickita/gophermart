package auth

import (
	"net/http"

	"github.com/iurnickita/gophermart/internal/store"
	"github.com/iurnickita/gophermart/internal/token"
)

type Auth interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	Middleware(h http.HandlerFunc) http.HandlerFunc
}

const (
	UserCodeKey     = "userCode"
	cookieUserToken = "gophermartUserToken"
)

type auth struct {
	store store.Store
}

func NewAuth(store store.Store) Auth {
	return &auth{store: store}
}

func (a *auth) Register(w http.ResponseWriter, r *http.Request) {

}

func (a *auth) Login(w http.ResponseWriter, r *http.Request) {

}

func (a *auth) Middleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получение id пользователя
		userCode, err := a.getUserCode(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// записываем
		r.Header.Set(UserCodeKey, userCode)

		// передаём управление хендлеру
		h.ServeHTTP(w, r)
	}
}

func (a *auth) getUserCode(_ http.ResponseWriter, r *http.Request) (string, error) {

	// куки пользователя
	var userCode string
	tokenCookie, err := r.Cookie(cookieUserToken)
	if err != nil {
		return "", err
	}
	userCode, err = token.GetUserCode(tokenCookie.Value)
	if err != nil {
		return "", err
	}
	return userCode, nil
}
