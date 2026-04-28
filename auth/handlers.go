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

	token, granted := s.Granted[handle]
	if !granted {
		if err := s.CheckGrant(handle); err != nil {
			return nil, handle, fmt.Errorf("credenciais inválidas")
		}
		token, granted = s.Granted[handle]
		if !granted {
			return nil, handle, fmt.Errorf("credenciais inválidas")
		}
	}
	if !s.Check(token, password) {
		return nil, handle, fmt.Errorf("credenciais inválidas")
	}

	if _, ok := s.HandleToToken[handle]; !ok {
		s.HandleToToken[handle] = token
		s.TokenToHandle[token] = handle
	}

	cookie, err := s.CreateSession(handle)
	return cookie, handle, err
}
