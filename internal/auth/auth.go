package auth

import (
	"bytes"
	"context"
	"encoding/json"
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
	HeaderUserCodeKey = "userCode"
	cookieUserToken   = "gophermartUserToken"
)

type auth struct {
	store store.Store
}

func NewAuth(store store.Store) (Auth, error) {
	return &auth{store: store}, nil
}

type RegisterJSONRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (a *auth) Register(w http.ResponseWriter, r *http.Request) {
	// Чтение логина/пароля
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var register RegisterJSONRequest
	err = json.Unmarshal(buffer.Bytes(), &register)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Запись в БД
	ctx := context.Background()
	userCode, err := a.store.AuthRegister(ctx, register.Login, register.Password)
	if err != nil {
		switch err {
		case store.ErrAlreadyExists:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Запись ID в JWT-токен
	a.setUserCode(w, r, userCode)
}

func (a *auth) Login(w http.ResponseWriter, r *http.Request) {
	// Чтение логина/пароля
	var buffer bytes.Buffer
	_, err := buffer.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var register RegisterJSONRequest
	err = json.Unmarshal(buffer.Bytes(), &register)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Проверка в БД
	ctx := context.Background()
	userCode, err := a.store.AuthLogin(ctx, register.Login, register.Password)
	if err != nil {
		switch err {
		case store.ErrNoRows:
			http.Error(w, "", http.StatusUnauthorized)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Запись ID в JWT-токен
	a.setUserCode(w, r, userCode)
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
		r.Header.Set(HeaderUserCodeKey, userCode)

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
	// извлечение из токена
	userCode, err = token.GetUserCode(tokenCookie.Value)
	if err != nil {
		return "", err
	}
	return userCode, nil
}

func (a *auth) setUserCode(w http.ResponseWriter, _ *http.Request, userCode string) error {

	// запись в токен
	tokenString, err := token.BuildJWTString(userCode)
	if err != nil {
		return err
	}
	// куки пользователя
	tokenCookie := http.Cookie{
		Name:  cookieUserToken,
		Value: tokenString,
	}
	http.SetCookie(w, &tokenCookie)
	return nil
}
