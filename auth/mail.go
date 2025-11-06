package auth

import (
	"fmt"
	"log"
	"net/smtp"

	"github.com/freehandle/breeze/crypto"
)

type MessagesTemplates struct {
	Reset                      string
	ResetHeader                string
	Signin                     string
	SigninHeader               string
	SigninWithoutHandle        string
	SigninWithoutHandleHeader  string
	Wellcome                   string
	WellcomeHeader             string
	EmailSigninMessage         string
	EmailSigninMessageHeader   string
	EmailPasswordMessage       string
	EmailPasswordMessageHeader string
	PasswordMessage            string
	PasswordMessageHeader      string
}

type Mailer interface {
	Send(to, subject, body string) bool
}

type SMTPGmail struct {
	Password string
	From     string
}

type TesteGmail struct{}

func (t TesteGmail) Send(to, subject, body string) bool {
	fmt.Printf("To: %s\nSubject: %s\n\n%s\n", to, subject, body)
	return true
}

func (s *SMTPGmail) Send(to, subject, body string) bool {
	auth := smtp.PlainAuth("", s.From, s.Password, "smtp.gmail.com")
	emailMsg := fmt.Sprintf("To: %s\r\n"+"Subject: %s\r\n"+"\r\n"+"%s\r\n", to, subject, body)
	err := smtp.SendMail("smtp.gmail.com:587", auth, s.From, []string{to}, []byte(emailMsg))
	if err != nil {
		log.Printf("email sending error: %v", err)
		return false
	}
	return true
}

type SMTPManager struct {
	Mail      Mailer
	Token     crypto.Token
	Templates MessagesTemplates
}

func (s *SMTPManager) SendReset(to, resetlink string, confirm chan bool) {
	go func() {
		check := s.Mail.Send(to, s.Templates.ResetHeader, fmt.Sprintf(s.Templates.Reset, resetlink))
		if confirm != nil {
			confirm <- check
		}
	}()
}

func (s *SMTPManager) SendSigninEmail(handle, email, fingerprint string, withoutHandle bool, confirm chan bool) {

	go func() {
		var check bool
		if withoutHandle {
			body := fmt.Sprintf(s.Templates.SigninWithoutHandle, handle, s.Token.Hex(), fingerprint)
			check = s.Mail.Send(email, s.Templates.SigninWithoutHandleHeader, body)
		} else {
			body := fmt.Sprintf(s.Templates.Signin, handle, s.Token.Hex(), fingerprint)
			check = s.Mail.Send(email, s.Templates.SigninHeader, body)
		}
		if confirm != nil {
			confirm <- check
		}
	}()
}

func (s *SMTPManager) SendPasswordEmail(handle, email, password string, confirm chan bool) {
	body := fmt.Sprintf(s.Templates.PasswordMessage, handle, password)
	go func() {
		check := s.Mail.Send(email, s.Templates.PasswordMessageHeader, body)
		if confirm != nil {
			confirm <- check
		}
	}()
}

func (s *SMTPManager) SendWellcome(handle, email string, confirm chan bool) {
	body := fmt.Sprintf(s.Templates.Wellcome, handle, handle, handle, handle, handle)
	go func() {
		check := s.Mail.Send(email, s.Templates.WellcomeHeader, body)
		if confirm != nil {
			confirm <- check
		}
	}()
}
