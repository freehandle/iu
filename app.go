package main

import (
	"fmt"
	"net/http"
	"time"
)

type Publicação struct {
	Quem      string    // perfil de quem publicou
	Quando    time.Time // quando publicou
	Que       string    // o que publicou
	Onde      string    // onde publicou (opcional)
	Interação Interação // interação com a publicação (opcional)
}

type Relacionamento struct {
	Agente         string
	Quando         time.Time
	Onde           string
	Relacionamento Relação
}

type Relação interface {
	Tipo() string
	Com() string
}

type Interação interface {
	Com() *Publicação
	Tipo() string
	Visible() bool
}

type Respositório interface {
	Rank(usuario, escopo string) ([]*Publicação, []*Relacionamento)
	Validar(p *Publicação) bool
	Agenciar(r *Relacionamento) bool
}

// Renderiza uma publicação, suas interações e relacionamentos
type Renderer interface {
	Render(publicação *Publicação, interações []*Publicação, relacionamentos []*Relacionamento) string //HTML
}

type Layout interface {
	Render(raiz *Publicação, publicacoes []*Publicação, relacionamentos []*Relacionamento) string
}

type App struct {
	Renderer             Renderer
	Layout               []Layout
	Repositório          Respositório
	ParserPost           func(*http.Request) (*Publicação, error)
	ParserRelacionamento func(*http.Request) (*Relacionamento, error)
}

func (a *App) Run(port int) {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", a.handleMain)
	http.HandleFunc("/api/posts", a.handlePublicação)
	http.HandleFunc("/api/relacionamentos", a.handleRelacionamento)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

}

func (a *App) handleMain(w http.ResponseWriter, r *http.Request) {
	posts, relacionamentos := a.Repositório.Rank("rubis", "raiz")
	if len(posts) == 0 {
		w.Write([]byte("Nenhum post encontrado"))
	} else {
		w.Write([]byte(a.Layout[0].Render(nil, posts, relacionamentos)))
	}

}

func (a *App) handlePublicação(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Erro ao processar dados", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	username := r.FormValue("username")

	if content == "" || username == "" {
		http.Error(w, "Conteúdo e usuário são obrigatórios", http.StatusBadRequest)
		return
	}

	newPost, err := a.ParserPost(r)
	// Validar post
	if err != nil {
		http.Error(w, fmt.Sprintf("Post inválido: %v", err), http.StatusBadRequest)
		return
	}

	// Validar post
	if !a.Repositório.Validar(newPost) {
		http.Error(w, "Post inválido", http.StatusBadRequest)
		return
	}

	// Responder com sucesso
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status": "success", "message": "Post criado com sucesso"}`))
}

func (a *App) handleRelacionamento(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Erro ao processar dados", http.StatusBadRequest)
		return
	}

	relacionamento, err := a.ParserRelacionamento(r)
	// Validar relacionamento
	if err != nil {
		http.Error(w, fmt.Sprintf("Relacionamento inválido: %v", err), http.StatusBadRequest)
		return
	}

	if !a.Repositório.Agenciar(relacionamento) {
		http.Error(w, "Relacionamento inválido", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "success", "message": "Usuário seguido com sucesso"}`))
}
