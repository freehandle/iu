package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
)

const cookieLifeItemSeconds = 60 * 60 * 24 * 7 // 1 week

func newCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   cookieLifeItemSeconds,
		Secure:   true,
		HttpOnly: true,
	}
}

func (s *SigninManager) CreateSession(handle string) (*http.Cookie, error) {
	_, ok := s.Granted[handle]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	seed := make([]byte, 32)
	if n, err := rand.Read(seed); n != 32 || err != nil {
		return nil, fmt.Errorf("error generating session cookie: %v", err)
	}
	cookie := hex.EncodeToString(seed)
	return newCookie(s.Members.AppName(), cookie), nil
}

func (s *SigninManager) CredentialsHandler(r *http.Request) (*http.Cookie, string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, "", err
	}
	handle := r.FormValue("handle")
	password := r.FormValue("password")
	token, ok := s.Granted[handle] //s.Members.Has(handle)
	fmt.Println("handle:", handle)
	fmt.Println("password:", password)
	fmt.Println("token:", token)
	if !ok || !s.Check(token, password) {
		var valid error
		if token, ok := s.Granted[handle]; ok {
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
