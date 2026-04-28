package credentials_test

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/iu/auth"
	"github.com/freehandle/iu/credentials"
)

// --- helpers de teste ---

type mockMailer struct{}

func (m *mockMailer) Send(to, subject, body string) bool { return true }

// novoManager cria um SigninManager pronto para testes usando arquivos temporários.
func novoManager(t *testing.T) (*auth.SigninManager, func()) {
	t.Helper()
	dir := t.TempDir()
	token, _ := crypto.RandomAsymetricKey()

	passwords := auth.NewFilePasswordManager(filepath.Join(dir, "passwords.dat"))
	cookies, err := auth.OpenCokieStore(filepath.Join(dir, "cookies.dat"))
	if err != nil {
		t.Fatal(err)
	}

	manager := &auth.SigninManager{
		AppName:       "TEST",
		AppToken:      token,
		Passwords:     passwords,
		Cookies:       cookies,
		Granted:       make(map[string]crypto.Token),
		HandleToToken: make(map[string]crypto.Token),
		TokenToHandle: make(map[crypto.Token]string),
		Invitation:    make(map[crypto.Hash]struct{}),
		Members:       &auth.DefaultAssociater{AplicationName: "TEST", AppToken: token},
		Mail: &auth.SMTPManager{
			Mail:  &mockMailer{},
			Token: token,
		},
	}

	cleanup := func() {
		passwords.Close()
		cookies.Close()
	}
	return manager, cleanup
}

// novoTemplates cria templates mínimos para todos os nomes usados pelos handlers.
func novoTemplates(t *testing.T) *template.Template {
	t.Helper()
	nomes := []string{
		"login.html", "signin.html", "invite.html",
		"forgot.html", "reset.html", "changepassword.html",
	}
	tmpl := template.New("")
	for _, nome := range nomes {
		template.Must(tmpl.New(nome).Parse(`ERROR:{{.Error}} SEED:{{.Seed}} HANDLE:{{.Handle}}`))
	}
	return tmpl
}

// novoMux cria um CredentialsMux de teste sem log de convites.
func novoMux(t *testing.T, manager *auth.SigninManager) *credentials.CredentialsMux {
	t.Helper()
	return credentials.New(manager, map[string]string{}, novoTemplates(t), "http://localhost", false, "")
}

// --- testes do InviteLog ---

func TestInviteLogNovo(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")
	il, err := credentials.AbrirInviteLog(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer il.Close()

	if len(il.Registros()) != 0 {
		t.Error("log novo deve estar vazio")
	}
}

func TestRegistrarConvite(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")
	il, _ := credentials.AbrirInviteLog(caminho)
	defer il.Close()

	il.RegistrarConvite("seed1", "@joao", "JORNAL")

	registros := il.Registros()
	if len(registros) != 1 {
		t.Fatalf("esperado 1 registro, got %d", len(registros))
	}
	r := registros[0]
	if r.Seed != "seed1" {
		t.Errorf("seed incorreto: %s", r.Seed)
	}
	if r.InviterHandle != "@joao" {
		t.Errorf("inviter incorreto: %s", r.InviterHandle)
	}
	if r.AppName != "JORNAL" {
		t.Errorf("appname incorreto: %s", r.AppName)
	}
	if r.CriadoEm.IsZero() {
		t.Error("CriadoEm não deve ser zero")
	}
	if !r.Pendente() {
		t.Error("convite recém-criado deve estar pendente")
	}
}

func TestRegistrarAceite(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")
	il, _ := credentials.AbrirInviteLog(caminho)
	defer il.Close()

	il.RegistrarConvite("seed1", "@joao", "JORNAL")
	il.RegistrarAceite("seed1", "@pedro")

	registros := il.Registros()
	if len(registros) != 1 {
		t.Fatalf("esperado 1 registro, got %d", len(registros))
	}
	r := registros[0]
	if r.Pendente() {
		t.Error("convite não deve estar pendente após aceite")
	}
	if r.InviteeHandle != "@pedro" {
		t.Errorf("invitee incorreto: %s", r.InviteeHandle)
	}
	if r.AceitoEm.IsZero() {
		t.Error("AceitoEm não deve ser zero")
	}
}

func TestAceiteConviteInexistente(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")
	il, _ := credentials.AbrirInviteLog(caminho)
	defer il.Close()

	// aceite de seed que nunca foi criado não deve causar pânico
	il.RegistrarAceite("seed_inexistente", "@alguem")

	if len(il.Registros()) != 0 {
		t.Error("aceite de seed inexistente não deve criar registro")
	}
}

func TestPersistencia(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")

	// sessão 1: grava dois convites, aceita um
	il, _ := credentials.AbrirInviteLog(caminho)
	il.RegistrarConvite("seed1", "@alice", "APP")
	il.RegistrarConvite("seed2", "@bob", "APP")
	il.RegistrarAceite("seed1", "@carol")
	il.Close()

	// sessão 2: recarrega e verifica
	il2, err := credentials.AbrirInviteLog(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer il2.Close()

	registros := il2.Registros()
	if len(registros) != 2 {
		t.Fatalf("esperado 2 registros após reload, got %d", len(registros))
	}

	porSeed := make(map[string]*credentials.RegistroConvite)
	for _, r := range registros {
		porSeed[r.Seed] = r
	}

	r1, ok := porSeed["seed1"]
	if !ok {
		t.Fatal("seed1 não encontrado após reload")
	}
	if r1.Pendente() {
		t.Error("seed1 deve estar aceito após reload")
	}
	if r1.InviteeHandle != "@carol" {
		t.Errorf("invitee incorreto após reload: %s", r1.InviteeHandle)
	}
	if r1.InviterHandle != "@alice" {
		t.Errorf("inviter incorreto após reload: %s", r1.InviterHandle)
	}

	r2, ok := porSeed["seed2"]
	if !ok {
		t.Fatal("seed2 não encontrado após reload")
	}
	if !r2.Pendente() {
		t.Error("seed2 deve estar pendente após reload")
	}
}

func TestArquivoTexto(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "convites.txt")
	il, _ := credentials.AbrirInviteLog(caminho)
	il.RegistrarConvite("seeda", "@joao", "MEGA")
	il.RegistrarAceite("seeda", "@maria")
	il.Close()

	// lê como texto simples e verifica o conteúdo legível
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	texto := string(conteudo)
	if !strings.Contains(texto, "CRIADO") {
		t.Error("arquivo deve conter linha CRIADO")
	}
	if !strings.Contains(texto, "ACEITO") {
		t.Error("arquivo deve conter linha ACEITO")
	}
	if !strings.Contains(texto, "@joao") {
		t.Error("arquivo deve conter inviter")
	}
	if !strings.Contains(texto, "@maria") {
		t.Error("arquivo deve conter invitee")
	}
	if !strings.Contains(texto, "MEGA") {
		t.Error("arquivo deve conter appname")
	}
}

// --- testes dos handlers ---

func TestLoginHandler(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
}

func TestForgotHandler(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/forgot", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
}

func TestChangePasswordHandlerGet(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/changepassword", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
}

func TestOnboardingHandlerModoAberto(t *testing.T) {
	// len(Invitation) == 0: qualquer seed é aceito
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/signin/qualquerhash", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "qualquerhash") {
		t.Error("seed deve aparecer no template")
	}
}

func TestOnboardingHandlerConviteInvalido(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()

	// adiciona um convite para ativar a validação
	tkFalso, _ := crypto.RandomAsymetricKey()
	manager.Invitation[crypto.HashToken(tkFalso)] = struct{}{}

	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/signin/hashquenaoexiste", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "convite inválido") {
		t.Error("deve mostrar mensagem de convite inválido")
	}
}

func TestOnboardingHandlerConviteValido(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()

	// gera um convite real via manager.Invite()
	seed := manager.Invite()

	// adiciona outro convite qualquer para ativar validação (map não vazio)
	tkFalso, _ := crypto.RandomAsymetricKey()
	manager.Invitation[crypto.HashToken(tkFalso)] = struct{}{}

	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/signin/"+seed, nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "convite inválido") {
		t.Error("convite válido não deve mostrar erro")
	}
}

func TestInviteHandlerSemSessao(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/invite", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "preciso estar logado") {
		t.Error("deve mostrar mensagem de não logado")
	}
}

func TestSignoutHandler(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/signout", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("esperado 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("esperado redirect para /, got %s", loc)
	}
}

func TestResetRequestSempreRedireciona(t *testing.T) {
	// deve redirecionar independente de o handle existir ou não
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	form := url.Values{}
	form.Set("handle", "@inexistente")
	form.Set("email", "nao@existe.com")

	req := httptest.NewRequest(http.MethodPost, "/resetrequest", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("esperado 303 sempre, got %d", w.Code)
	}
}

func TestResetFromURLInvalido(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()
	mux := novoMux(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/r/urlquenaoexiste", nil)
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "link para troca de senha inválido") {
		t.Error("deve mostrar mensagem de link inválido")
	}
}

func TestChangePasswordSenhasNaoBatem(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()

	token, _ := crypto.RandomAsymetricKey()
	manager.HandleToToken["@usuario"] = token
	manager.Set(token, "senhavelha", "email@test.com")

	mux := novoMux(t, manager)

	form := url.Values{}
	form.Set("handle", "@usuario")
	form.Set("oldpassword", "senhavelha")
	form.Set("newpassword", "novasenha1")
	form.Set("repeatnewpassword", "novasenha2") // senhas diferentes

	req := httptest.NewRequest(http.MethodPost, "/changepassword", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperado 200 com erro, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "senhas novas não bate") {
		t.Error("deve mostrar mensagem de senhas diferentes")
	}
}

func TestChangePasswordSucesso(t *testing.T) {
	manager, cleanup := novoManager(t)
	defer cleanup()

	token, _ := crypto.RandomAsymetricKey()
	manager.HandleToToken["@usuario"] = token
	manager.Set(token, "senhavelha", "email@test.com")

	mux := novoMux(t, manager)

	form := url.Values{}
	form.Set("handle", "@usuario")
	form.Set("oldpassword", "senhavelha")
	form.Set("newpassword", "novasenha")
	form.Set("repeatnewpassword", "novasenha")

	req := httptest.NewRequest(http.MethodPost, "/changepassword", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("esperado 303 redirect, got %d", w.Code)
	}
	// verifica que a nova senha funciona
	if !manager.Check(token, "novasenha") {
		t.Error("nova senha deve ser válida após troca")
	}
}

func TestRegisterHandlerComSafeAPIMock(t *testing.T) {
	userToken, _ := crypto.RandomAsymetricKey()

	// mock do Safe API
	safeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"status": "novo",
			"token":  userToken.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer safeServer.Close()

	manager, cleanup := novoManager(t)
	defer cleanup()
	manager.SafeAPIAddress = safeServer.URL

	seed := manager.Invite()
	mux := novoMux(t, manager)

	form := url.Values{}
	form.Set("handle", "@novousuario")
	form.Set("email", "novo@test.com")
	form.Set("password", "senha123")
	form.Set("seed", seed)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.Mux.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("esperado 303 redirect para /login, got %d — body: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("esperado redirect para /login, got %s", loc)
	}
	// convite deve ter sido consumido
	if len(manager.Invitation) != 0 {
		t.Error("convite deve ter sido removido após cadastro")
	}
}

func TestInviteLogIntegrado(t *testing.T) {
	// verifica que InviteHandler e RegisterHandler gravam no log
	manager, cleanup := novoManager(t)
	defer cleanup()

	userToken, _ := crypto.RandomAsymetricKey()
	safeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"status": "novo", "token": userToken.String()}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer safeServer.Close()
	manager.SafeAPIAddress = safeServer.URL

	// cria sessão para @inviter poder usar /invite
	inviterToken, _ := crypto.RandomAsymetricKey()
	manager.HandleToToken["@inviter"] = inviterToken
	manager.TokenToHandle[inviterToken] = "@inviter"
	manager.Granted["@inviter"] = inviterToken
	cookieVal := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64 hex chars
	manager.Cookies.Set(inviterToken, cookieVal, 0)

	logPath := filepath.Join(t.TempDir(), "convites.txt")
	mux := credentials.New(manager, map[string]string{}, novoTemplates(t), "http://localhost", false, logPath)

	// --- passo 1: gera convite ---
	reqInvite := httptest.NewRequest(http.MethodGet, "/invite", nil)
	reqInvite.AddCookie(&http.Cookie{Name: "TEST", Value: cookieVal})
	wInvite := httptest.NewRecorder()
	mux.Mux.ServeHTTP(wInvite, reqInvite)

	if wInvite.Code != http.StatusOK {
		t.Fatalf("esperado 200 no invite, got %d", wInvite.Code)
	}

	// extrai o seed do body do template (SEED:...)
	body := wInvite.Body.String()
	seedStart := strings.Index(body, "SEED:") + 5
	seedEnd := strings.Index(body[seedStart:], " ")
	if seedEnd < 0 {
		seedEnd = len(body[seedStart:])
	}
	seed := body[seedStart : seedStart+seedEnd]

	// --- passo 2: aceita convite ---
	form := url.Values{}
	form.Set("handle", "@convidado")
	form.Set("email", "convidado@test.com")
	form.Set("password", "senha123")
	form.Set("seed", seed)

	reqReg := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	reqReg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wReg := httptest.NewRecorder()
	mux.Mux.ServeHTTP(wReg, reqReg)

	// --- verifica log ---
	il, err := credentials.AbrirInviteLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer il.Close()

	registros := il.Registros()
	if len(registros) != 1 {
		t.Fatalf("esperado 1 registro no log, got %d", len(registros))
	}
	r := registros[0]
	if r.InviterHandle != "@inviter" {
		t.Errorf("inviter incorreto: %s", r.InviterHandle)
	}
	if r.InviteeHandle != "@convidado" {
		t.Errorf("invitee incorreto: %s", r.InviteeHandle)
	}
	if r.Pendente() {
		t.Error("convite deve estar aceito após registro")
	}
}
