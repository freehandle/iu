package credentials

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/iu/auth"
)

// CredentialsMux agrupa os handlers de autenticação com mux próprio.
// Rotas registradas:
//
//	GET  /login            — formulário de login
//	POST /credentials      — processa login
//	GET  /signin/{seed}    — formulário de cadastro (valida convite)
//	POST /register         — processa cadastro
//	GET  /invite           — gera link de convite (requer sessão)
//	GET  /forgot           — formulário "esqueci minha senha"
//	POST /resetrequest     — envia email de reset
//	GET  /r/{url}          — formulário de nova senha via link
//	POST /reset            — processa nova senha via link
//	GET  /changepassword   — formulário de troca de senha autenticada
//	POST /changepassword   — processa troca de senha
//	GET  /signout          — encerra sessão
type CredentialsMux struct {
	Manager            *auth.SigninManager
	Templates          *template.Template
	Hostname           string // base URL, ex: "http://localhost:8030"
	RedirectAfterLogin string // rota pós-login bem-sucedido, padrão "/"
	SecureCookie       bool   // false para HTTP local, true para HTTPS produção
	Log                *InviteLog // nil desativa o registro de convites
	Mux                *http.ServeMux
}

// View é o dado enviado aos templates de credenciais.
type View struct {
	AppName string
	Error   string
	Handle  string
	Seed    string
	URL     string
}

// New cria e registra todas as rotas no Mux interno.
// logPath é o caminho do arquivo de texto de registro de convites; se vazio, o log é desativado.
func New(manager *auth.SigninManager, templates *template.Template, hostname string, secureCookie bool, logPath string) *CredentialsMux {
	var il *InviteLog
	if logPath != "" {
		var err error
		il, err = AbrirInviteLog(logPath)
		if err != nil {
			log.Printf("credentials: não foi possível abrir invite log: %v", err)
		}
	}
	c := &CredentialsMux{
		Manager:            manager,
		Templates:          templates,
		Hostname:           hostname,
		RedirectAfterLogin: "/",
		SecureCookie:       secureCookie,
		Log:                il,
		Mux:                http.NewServeMux(),
	}
	c.Mux.HandleFunc("/login", c.LoginHandler)
	c.Mux.HandleFunc("/credentials", c.CredentialsHandler)
	c.Mux.HandleFunc("/signin/", c.OnboardingHandler)
	c.Mux.HandleFunc("/register", c.RegisterHandler)
	c.Mux.HandleFunc("/invite", c.InviteHandler)
	c.Mux.HandleFunc("/forgot", c.ForgotHandler)
	c.Mux.HandleFunc("/resetrequest", c.ResetRequestHandler)
	c.Mux.HandleFunc("/r/", c.ResetFromURLHandler)
	c.Mux.HandleFunc("/reset", c.ResetHandler)
	c.Mux.HandleFunc("/changepassword", c.ChangePasswordHandler)
	c.Mux.HandleFunc("/signout", c.SignoutHandler)
	return c
}

func (c *CredentialsMux) render(w http.ResponseWriter, tmpl string, view View) {
	if err := c.Templates.ExecuteTemplate(w, tmpl, view); err != nil {
		log.Println("credentials render:", err)
	}
}

// setCookie aplica o cookie na resposta, respeitando SecureCookie.
func (c *CredentialsMux) setCookie(w http.ResponseWriter, cookie *http.Cookie) {
	cookie.Secure = c.SecureCookie
	http.SetCookie(w, cookie)
}

// LoginHandler exibe o formulário de login.
func (c *CredentialsMux) LoginHandler(w http.ResponseWriter, r *http.Request) {
	c.render(w, "login.html", View{AppName: c.Manager.AppName})
}

// CredentialsHandler processa o formulário de login (POST /credentials).
func (c *CredentialsMux) CredentialsHandler(w http.ResponseWriter, r *http.Request) {
	cookie, _, err := c.Manager.CredentialsHandler(r)
	if err != nil {
		c.render(w, "login.html", View{Error: err.Error(), AppName: c.Manager.AppName})
		return
	}
	c.setCookie(w, cookie)
	http.Redirect(w, r, c.RedirectAfterLogin, http.StatusSeeOther)
}

// OnboardingHandler exibe o formulário de cadastro validando o hash de convite.
func (c *CredentialsMux) OnboardingHandler(w http.ResponseWriter, r *http.Request) {
	seed := strings.TrimPrefix(r.URL.Path, "/signin/")
	hash := crypto.DecodeHash(seed)
	if _, ok := c.Manager.Invitation[hash]; ok || len(c.Manager.Invitation) == 0 {
		c.render(w, "signin.html", View{Seed: seed, AppName: c.Manager.AppName})
		return
	}
	c.render(w, "login.html", View{Error: "convite inválido", AppName: c.Manager.AppName})
}

// RegisterHandler processa o formulário de cadastro (POST /register).
func (c *CredentialsMux) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.render(w, "signin.html", View{Error: "erro ao processar formulário", AppName: c.Manager.AppName})
		return
	}
	handle := r.FormValue("handle")
	email := r.FormValue("email")
	passwd := r.FormValue("password")
	seed := r.FormValue("seed")
	hash := crypto.DecodeHash(seed)
	if !c.Manager.OnboardSigner(handle, email, passwd) {
		c.render(w, "signin.html", View{Error: "perfil já existente ou erro no cadastro", Seed: seed, AppName: c.Manager.AppName})
		return
	}
	if c.Log != nil {
		c.Log.RegistrarAceite(seed, handle)
	}
	delete(c.Manager.Invitation, hash)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// InviteHandler gera um link de convite e o exibe (requer sessão ativa).
func (c *CredentialsMux) InviteHandler(w http.ResponseWriter, r *http.Request) {
	handle, _ := c.Manager.SessionUser(r)
	if handle == "" {
		c.render(w, "login.html", View{Error: "é preciso estar logado para convidar", AppName: c.Manager.AppName})
		return
	}
	seed := c.Manager.Invite()
	if c.Log != nil {
		c.Log.RegistrarConvite(seed, handle, c.Manager.AppName)
	}
	c.render(w, "invite.html", View{Seed: seed, Handle: handle, AppName: c.Manager.AppName})
}

// ForgotHandler exibe o formulário "esqueci minha senha".
func (c *CredentialsMux) ForgotHandler(w http.ResponseWriter, r *http.Request) {
	c.render(w, "forgot.html", View{AppName: c.Manager.AppName})
}

// ResetRequestHandler processa o formulário "esqueci minha senha" e envia o email de reset.
func (c *CredentialsMux) ResetRequestHandler(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/login", http.StatusSeeOther)
	if err := r.ParseForm(); err != nil {
		return
	}
	handle := r.FormValue("handle")
	email := r.FormValue("email")
	token, ok := c.Manager.HandleToToken[handle]
	if ok {
		c.Manager.RequestReset(token, email, c.Hostname)
	}
	// redireciona sempre para não revelar se handle/email existem
}

// ResetFromURLHandler exibe o formulário de nova senha a partir do link de reset.
func (c *CredentialsMux) ResetFromURLHandler(w http.ResponseWriter, r *http.Request) {
	resetURL := strings.TrimPrefix(r.URL.Path, "/r/")
	ok, token, _ := c.Manager.Passwords.HasReset(resetURL)
	if !ok {
		c.render(w, "login.html", View{Error: "link para troca de senha inválido", AppName: c.Manager.AppName})
		return
	}
	handle := c.Manager.TokenToHandle[token]
	c.render(w, "reset.html", View{Handle: handle, URL: r.URL.Path, AppName: c.Manager.AppName})
}

// ResetHandler processa o formulário de nova senha via link (POST /reset).
func (c *CredentialsMux) ResetHandler(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/login", http.StatusSeeOther)
	if err := r.ParseForm(); err != nil {
		return
	}
	resetURL := strings.TrimPrefix(r.FormValue("url"), "/r/")
	password := r.FormValue("password")
	handle := r.FormValue("handle")
	token, ok := c.Manager.HandleToToken[handle]
	if ok {
		c.Manager.Reset(token, resetURL, password)
	}
}

// ChangePasswordHandler exibe (GET) e processa (POST) a troca de senha autenticada.
func (c *CredentialsMux) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		c.render(w, "changepassword.html", View{AppName: c.Manager.AppName})
		return
	}
	if err := r.ParseForm(); err != nil {
		c.render(w, "changepassword.html", View{Error: "erro ao processar formulário", AppName: c.Manager.AppName})
		return
	}
	handle := r.FormValue("handle")
	oldpassword := r.FormValue("oldpassword")
	newpassword := r.FormValue("newpassword")
	repeatnew := r.FormValue("repeatnewpassword")
	token, ok := c.Manager.HandleToToken[handle]
	if !ok || !c.Manager.Check(token, oldpassword) {
		c.render(w, "changepassword.html", View{Error: "credenciais incorretas", AppName: c.Manager.AppName})
		return
	}
	if newpassword != repeatnew {
		c.render(w, "changepassword.html", View{Error: "o campo das senhas novas não bate", AppName: c.Manager.AppName})
		return
	}
	if !c.Manager.DirectReset(token, newpassword) {
		c.render(w, "changepassword.html", View{Error: "não foi possível atualizar a senha", AppName: c.Manager.AppName})
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// SignoutHandler encerra a sessão do usuário e redireciona para "/".
func (c *CredentialsMux) SignoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(c.Manager.AppName); err == nil {
		_, token := c.Manager.SessionUser(r)
		c.Manager.Cookies.Unset(token, cookie.Value)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
