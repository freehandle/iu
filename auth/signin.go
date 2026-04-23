package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"
	"github.com/freehandle/handles/attorney"
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

type ManagerConfig struct {
	Token     crypto.Token
	Passwords PasswordManager
	Mail      Mailer
	Gateway   Gateway
	Templates MessagesTemplates
	Source    chan []byte
}

func LaunchManager(ctx context.Context, config ManagerConfig, source chan []byte) (*SigninManager, chan []byte) {
	forward := make(chan []byte, 1)
	manager := NewSigninManager(config.Token, config.Passwords, config.Mail, config.Gateway, config.Templates)
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(forward)
				return
			case data, ok := <-source:
				if !ok {
					close(forward)
					return
				}
				switch data[0] {
				case 1:
					kind := attorney.Kind(data[1:])
					switch kind {
					case attorney.JoinNetworkType:
						if join := attorney.ParseJoinNetwork(data[1:]); join != nil {
							manager.HandleToToken[join.Handle] = join.Author
							manager.TokenToHandle[join.Author] = join.Handle
						}
					case attorney.GrantPowerOfAttorneyType:
						if grant := attorney.ParseGrantPowerOfAttorney(data[1:]); grant != nil {
							if grant.Attorney.Equal(config.Token) {
								if handle, ok := manager.TokenToHandle[grant.Author]; ok {
									manager.Granted[handle] = grant.Author
								}
							}
						}
					case attorney.RevokePowerOfAttorneyType:
						if revoke := attorney.ParseRevokePowerOfAttorney(data[1:]); revoke != nil {
							if revoke.Attorney.Equal(config.Token) {
								if handle, ok := manager.TokenToHandle[revoke.Author]; ok {
									delete(manager.Granted, handle)
								}
							}
						}
					case attorney.VoidType:
						forward <- data
					}
				case 0:
					if len(data) >= 9 {
						epoch, _ := util.ParseUint64(data[1:], 1)
						manager.Epoch = epoch
					}
					forward <- data
				default:
					forward <- data
				}
			}
		}
	}()
	return manager, forward
}

func NewSigninManager(token crypto.Token, passwords PasswordManager, mail Mailer, gateway Gateway, templates MessagesTemplates) *SigninManager {
	if gateway == nil {
		log.Print("PANIC BUG: NewSigninManager called with nil gateway ")
		return nil
	}
	return &SigninManager{
		//pending:   make([]*Signerin, 0),
		Passwords: passwords,
		//Gateway:   gateway,
		Mail:          &SMTPManager{Token: token, Mail: mail, Templates: templates},
		Granted:       make(map[string]crypto.Token),
		HandleToToken: make(map[string]crypto.Token),
		TokenToHandle: make(map[crypto.Token]string),
		Invitation:    make(map[crypto.Hash]struct{}),
	}
}

type Associater interface {
	//Has(handle string) (crypto.Token, bool)
	Invite(handle string, token crypto.Token) error
	AppName() string
	AttorneyToken() crypto.Token
}

// TODO IMPLEMENTAR LOGICA DO INVITE AQUI

func (d *DefaultAssociater) AppName() string {
	return d.AplicationName
}

func (d *DefaultAssociater) AttorneyToken() crypto.Token {
	return d.AppToken
}

func (d *DefaultAssociater) Invite(handle string, token crypto.Token) error {
	return nil
}

type DefaultAssociater struct {
	AplicationName string
	AppToken       crypto.Token
}

type SigninManager struct {
	Epoch     uint64
	AppName   string
	AppToken  crypto.Token
	Passwords PasswordManager
	Cookies   *CookieStore
	Mail      *SMTPManager
	//Gateway       Gateway
	Granted        map[string]crypto.Token
	Credentials    crypto.PrivateKey
	Members        Associater
	SafeAddress    string
	SafeAPIAddress string
	HandleToToken  map[string]crypto.Token
	TokenToHandle  map[crypto.Token]string
	Invitation     map[crypto.Hash]struct{}
}

func (s *SigninManager) OnboardSigner(handle, email, passwd string) bool {
	data := safe.UserRequest{
		Handle:        handle,
		Email:         email,
		Password:      passwd,
		App:           s.Members.AppName(),
		AttorneyToken: s.Members.AttorneyToken().String(),
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("error marshalling JSON:", err)
		return false
	}
	resp, err := http.Post(s.SafeAPIAddress, "application/json", bytes.NewBuffer(jsonData))
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
	fmt.Println("Onboarded user with token:", token.String())
	s.Set(token, passwd, email)
	s.Granted[handle] = token
	if response.Status != "existente" {
		if err := s.Members.Invite(handle, token); err != nil {
			log.Println("error inviting user to members:", err)
			return false
		}
	} else {
		check := response.Verify
		go s.Mail.SendVerifyPOA(handle, s.AppName, email, check)
	}
	return true
}

func (s *SigninManager) CheckGrant(handle string) error {
	req := safe.AttorneyRequest{
		Handle:        handle,
		AttorneyToken: s.Members.AttorneyToken().Hex(),
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error marshalling JSON:%s", err)
	}
	resp, err := http.Post(fmt.Sprintf("%s/attorney", s.SafeAPIAddress), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending onboarding request: %s", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading onboarding response body: %s", err)
	}
	var response safe.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error unmarshalling onboarding response: %s", err)
	}
	fmt.Println("Response status:", resp.Status)
	if resp.Status != "200 OK" {
		return fmt.Errorf("error onboarding user: %s", response.Message)
	}
	if response.Status == "Granted" {
		token := crypto.TokenFromString(response.Token)
		if err := s.Members.Invite(handle, token); err != nil {
			return fmt.Errorf("error inviting user to members: %s", err)
		}
		s.Granted[handle] = token
		return nil
	}
	return fmt.Errorf("access not granted")
}

func (s *SigninManager) RequestReset(user crypto.Token, email, domain string) bool {
	if !s.Passwords.HasWithEmail(user, email) {
		return false
	}
	reset := s.Passwords.AddReset(user, email)
	url := fmt.Sprintf("%s/r/%s", domain, reset)
	if reset == "" {
		return false
	}
	s.Mail.SendReset(email, url, nil)
	return true
}

func (s *SigninManager) Reset(user crypto.Token, url, password string) bool {
	return s.Passwords.DropReset(user, url, password)
}

func (s *SigninManager) Check(user crypto.Token, password string) bool {
	hashed := crypto.Hasher(append(user[:], []byte(password)...))
	return s.Passwords.Check(user, hashed)
}

func (s *SigninManager) Set(user crypto.Token, password string, email string) {
	hashed := crypto.Hasher(append(user[:], []byte(password)...))
	s.Passwords.Set(user, hashed, email)
}

func (s *SigninManager) DirectReset(user crypto.Token, newpassword string) bool {
	newhashed := crypto.Hasher(append(user[:], []byte(newpassword)...))
	return s.Passwords.Reset(user, newhashed)
}

func (s *SigninManager) Has(token crypto.Token) bool {
	return s.Passwords.Has(token)
}

func (s *SigninManager) Invite() string {
	tk, _ := crypto.RandomAsymetricKey()
	hash := crypto.HashToken(tk)
	s.Invitation[hash] = struct{}{}
	return hash.String()
}

func (s *SigninManager) Invited(hash string) bool {
	if len(s.Granted) == 0 {
		return true
	}
	parsed := crypto.DecodeHash(hash)
	_, ok := s.Invitation[parsed]
	return ok
}
