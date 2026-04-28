package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"github.com/freehandle/breeze/crypto"
)

const cookieLifeItemSeconds = 60 * 60 * 24 * 7 // 1 week

func (s *SigninManager) newCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     s.CookieName,
		Value:    url.QueryEscape(value),
		Path:     "/",
		MaxAge:   cookieLifeItemSeconds,
		Secure:   s.Secure,
		HttpOnly: true,
	}
}

func (s *SigninManager) SessionUser(r *http.Request) (string, crypto.Token) {
	cookie, err := r.Cookie(s.CookieName)
	if err != nil {
		return "", crypto.ZeroToken
	}
	value, _ := url.QueryUnescape(cookie.Value)
	if token, ok := s.Cookies.Get(value); ok {
		if handle, ok := s.TokenToHandle[token]; ok {
			return handle, token
		}
	}
	return "", crypto.ZeroToken
}

func (s *SigninManager) CreateSession(handle string) (*http.Cookie, error) {
	_, ok := s.Granted[handle]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	token, ok := s.HandleToToken[handle]
	if !ok {
		return nil, fmt.Errorf("user not member of the handles network")
	}
	seed := make([]byte, 32)
	if n, err := rand.Read(seed); n != 32 || err != nil {
		return nil, fmt.Errorf("error generating session cookie: %v", err)
	}
	cookie := hex.EncodeToString(seed)
	s.Cookies.Set(token, cookie, 0)
	return s.newCookie(cookie), nil
}

func (s *SigninManager) CredentialsHandler(r *http.Request) (*http.Cookie, string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, "", err
	}
	handle := r.FormValue("handle")
	password := r.FormValue("password")
	token, ok := s.Granted[handle] //s.Members.Has(handle)
	if !ok || !s.Check(token, password) {
		var valid error
		if token, ok := s.Granted[handle]; ok { //se passou pelo signin
			if s.Check(token, password) {
				valid = s.CheckGrant(handle)
			}
		}
		if valid != nil {
			return nil, handle, fmt.Errorf("pendente de aprovação pelo usuário: %s", valid)
		} else {
			token, ok := s.Granted[handle]
			if ok {
				s.Members.Invite(handle, token)
			} else {
				return nil, handle, fmt.Errorf("erro interno ao recuperar token concedido")
			}
		}
	}
	cookie, err := s.CreateSession(handle)
	return cookie, handle, err
}
