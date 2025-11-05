package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/safe"
)

type Signerin struct {
	Handle      string
	Email       string
	TimeStamp   time.Time
	FingerPrint string
}

type Attorney interface {
	Signin(handle, email, passwd string) bool
}

type Gateway interface {
	Send(action []byte)
	Epoch() uint64
}

func NewSigninManager(token crypto.Token, passwords PasswordManager, mail Mailer, gateway Gateway, templates MessagesTemplates) *SigninManager {
	if gateway == nil {
		log.Print("PANIC BUG: NewSigninManager called with nil gateway ")
		return nil
	}
	return &SigninManager{
		pending:   make([]*Signerin, 0),
		passwords: passwords,
		Gateway:   gateway,
		mail:      &SMTPManager{Token: token, Mail: mail, Templates: templates},
		Granted:   make(map[string]crypto.Token),
	}
}

type SigninManager struct {
	safe        int // for optional direct onboarding
	pending     []*Signerin
	passwords   PasswordManager
	mail        *SMTPManager
	Gateway     Gateway
	Granted     map[string]crypto.Token
	Credentials crypto.PrivateKey
	Confirm     chan string
}

func (s *SigninManager) OnboardSigner(handle, email, passwd string) bool {
	if s.safe == 0 {
		log.Println("PANIC BUG: OnboardSigner called with nil safe")
		return false
	}

	data := safe.UserRequest{Handle: handle, Email: email, Password: passwd}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("error marshalling JSON:", err)
		return false
	}
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d", s.safe), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("error sending onboarding request:", err)
		return false
	}
	var token crypto.Token
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading onboarding response body:", err)
		return false
	}
	var response safe.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Println("error unmarshalling onboarding response:", err)
		return false
	}
	if resp.Status != "200 OK" {
		log.Println("error onboarding user:", response.Message)
		return false
	}
	token = crypto.TokenFromString(response.Token)
	fmt.Println("Onboarded user with token:", token.String(), response.Token)
	grant := safe.GrantRequest{
		Handle:        handle,
		AttorneyToken: s.Credentials.PublicKey().String(),
		Hash:          crypto.EncodeHash(crypto.HashToken(token)),
	}
	jsonData, err = json.Marshal(grant)
	if err != nil {
		log.Println("error marshalling JSON:", err)
		return false
	}
	resp, err = http.Post(fmt.Sprintf("http://localhost:%d/grant", s.safe), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("error sending onboarding request:", err)
		return false
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error reading onboarding response body:", err)
		return false
	}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Println("error unmarshalling onboarding response:", err)
		return false
	}
	if resp.Status != "200 OK" {
		log.Println("error granting power of attorney:", response.Message)
		return false
	} else {
		log.Println("success granting power of attorney:", response.Message)
	}
	s.Set(token, passwd, email)
	s.Granted[handle] = token
	s.mail.SendWellcome(handle, email, nil)
	return true
}

func (s *SigninManager) RequestReset(user crypto.Token, email, domain string) bool {
	if !s.passwords.HasWithEmail(user, email) {
		return false
	}
	reset := s.passwords.AddReset(user, email)
	url := fmt.Sprintf("%s/r/%s", domain, reset)
	if reset == "" {
		return false
	}
	s.mail.SendReset(email, url, nil)
	return true
}

func (s *SigninManager) Reset(user crypto.Token, url, password string) bool {
	return s.passwords.DropReset(user, url, password)
}

func (s *SigninManager) Check(user crypto.Token, password string) bool {
	hashed := crypto.Hasher(append(user[:], []byte(password)...))
	return s.passwords.Check(user, hashed)
}

func (s *SigninManager) Set(user crypto.Token, password string, email string) {
	hashed := crypto.Hasher(append(user[:], []byte(password)...))
	s.passwords.Set(user, hashed, email)
}

func (s *SigninManager) DirectReset(user crypto.Token, newpassword string) bool {
	newhashed := crypto.Hasher(append(user[:], []byte(newpassword)...))
	return s.passwords.Reset(user, newhashed)
}

func (s *SigninManager) Has(token crypto.Token) bool {
	return s.passwords.Has(token)
}

func (s *SigninManager) AddSigner(handle, email string, token *crypto.Token) {
	signer := &Signerin{}
	for _, pending := range s.pending {
		if signer.Handle == handle {
			signer = pending
		}
	}
	signer.Handle = handle
	signer.Email = email
	t, _ := crypto.RandomAsymetricKey()
	signer.FingerPrint = crypto.EncodeHash(crypto.HashToken(t))
	signer.TimeStamp = time.Now()
	s.mail.SendSigninEmail(handle, email, signer.FingerPrint, token == nil, nil)
	s.pending = append(s.pending, signer)
}

func randomPassword() string {
	bytes := make([]byte, 10)
	rand.Read(bytes)
	return base64.StdEncoding.EncodeToString(bytes)
}

func (s *SigninManager) RevokeAttorney(handle string) {
	delete(s.Granted, handle)
}

func (s *SigninManager) GrantAttorney(token crypto.Token, handle, fingerprint string) {
	for n, signer := range s.pending {
		if signer.Handle == handle && signer.FingerPrint == fingerprint {
			passwd := randomPassword()
			s.Set(token, passwd, signer.Email)
			if s.Confirm != nil {
				s.Confirm <- handle
			}
			s.mail.SendPasswordEmail(signer.Handle, signer.Email, passwd, nil)
			s.pending = append(s.pending[:n], s.pending[n+1:]...)
			s.Granted[handle] = token
		}
	}
}
