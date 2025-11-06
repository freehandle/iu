package main

/*
import (
	"html/template"
	"net/http"
	"time"
)

// Adicionar método Visible() ao tipo Seguir para implementar a interface Interaction
func (s *Seguir) Visible() bool {
	return true
}

// TemplateRenderer é um renderer que usa templates HTML
type TemplateRenderer struct {
	template *template.Template
}

// NewTemplateRenderer cria um novo renderer de template
func NewTemplateRenderer() (*TemplateRenderer, error) {
	tmpl, err := template.ParseFiles("templates/timeline.html")
	if err != nil {
		return nil, err
	}
	return &TemplateRenderer{template: tmpl}, nil
}

// Render renderiza posts usando o template HTML
func (r *TemplateRenderer) Render(p *Publicação, interacoes []Interação) string {
	// Esta função seria chamada pelo sistema Go existente
	// Por enquanto, retornamos HTML básico que será substituído pelo template
	return ""
}

// AppWithTemplate é uma versão da App que usa templates HTML
type AppWithTemplate struct {
	renderer *TemplateRenderer
	state    Respositório
}

// NewAppWithTemplate cria uma nova aplicação com suporte a templates
func NewAppWithTemplate() (*AppWithTemplate, error) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		return nil, err
	}

	return &AppWithTemplate{
		renderer: renderer,
		state: &VibeApp{
			seguindo: make(map[string][]string),
			posts:    make([]*Publicação, 0),
			topicos:  []string{"lendo", "ouvindo", "namorando", "assistindo", "jogando", "preocupando", "comendo", "cobiçando"},
		},
	}, nil
}

// RunWithTemplate executa a aplicação com suporte a templates HTML
func (a *AppWithTemplate) RunWithTemplate(port int) error {
	// Handler para a página principal
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		posts := a.state.Rank("rubis")

		// Preparar dados para o template
		data := struct {
			Posts     []*Publicação
			Username  string
			Timestamp time.Time
		}{
			Posts:     posts,
			Username:  "rubis",
			Timestamp: time.Now(),
		}

		// Definir headers
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Renderizar template
		err := a.renderer.template.Execute(w, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Servir arquivos estáticos (CSS, JS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates"))))

	// Handler para criar novos posts (API)
	http.HandleFunc("/api/posts", func(w http.ResponseWriter, r *http.Request) {
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

		// Criar novo post
		newPost := &Publicação{
			Quem:      username,
			Quando:    time.Now(),
			Onde:      "",
			Que:       content,
			Interação: nil,
		}

		// Validar post
		if !a.state.Validar(newPost) {
			http.Error(w, "Post inválido", http.StatusBadRequest)
			return
		}

		// Responder com sucesso
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "success", "message": "Post criado com sucesso"}`))
	})

	// Handler para seguir usuários (API)
	http.HandleFunc("/api/follow", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Erro ao processar dados", http.StatusBadRequest)
			return
		}

		follower := r.FormValue("follower")
		following := r.FormValue("following")

		if follower == "" || following == "" {
			http.Error(w, "Follower e following são obrigatórios", http.StatusBadRequest)
			return
		}

		// Criar interação de seguir
		seguir := &Seguir{
			seguidor: follower,
			post:     &Publicação{Quem: following, Quando: time.Now(), Onde: "", Que: "", Interação: nil},
		}

		a.state.Update(seguir)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Usuário seguido com sucesso"}`))
	})

	// Handler para obter posts em JSON (API)
	http.HandleFunc("/api/posts-json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		username := r.URL.Query().Get("user")
		if username == "" {
			username = "rubis" // default
		}

		posts := a.state.Rank(username)

		// Converter posts para JSON simples
		postsData := make([]map[string]interface{}, len(posts))
		for i, post := range posts {
			postsData[i] = map[string]interface{}{
				"quem":   post.Quem,
				"quando": post.Quando.Format(time.RFC3339),
				"onde":   post.Onde,
				"que":    post.Que,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"posts": ` + postsToJSON(postsData) + `}`))
	})

	return http.ListenAndServe(":8000", nil)
}

// Função auxiliar para converter posts para JSON (simplificada)
func postsToJSON(posts []map[string]interface{}) string {
	// Implementação simplificada - em produção use encoding/json
	result := "["
	for i, post := range posts {
		if i > 0 {
			result += ","
		}
		result += `{"quem":"` + post["quem"].(string) + `",`
		result += `"quando":"` + post["quando"].(string) + `",`
		result += `"onde":"` + post["onde"].(string) + `",`
		result += `"que":"` + post["que"].(string) + `"}`
	}
	result += "]"
	return result
}

// Exemplo de uso
func runTemplateExample() {
	app, err := NewAppWithTemplate()
	if err != nil {
		panic(err)
	}

	// Adicionar alguns posts de exemplo
	postLari := &Publicação{
		Quem:      "lari",
		Quando:    time.Now().Add(-5 * time.Minute),
		Onde:      "",
		Que:       "lendo:murder in mesopotamia\nouvindo:dupê\njogando:monstruosas",
		Interação: nil,
	}

	postMaria := &Publicação{
		Quem:      "maria",
		Quando:    time.Now().Add(-15 * time.Minute),
		Onde:      "São Paulo",
		Que:       "assistindo:Stranger Things\ncomendo:pizza",
		Interação: nil,
	}

	app.state.Validar(postLari)
	app.state.Validar(postMaria)

	// Seguir usuários
	seguirLari := &Seguir{
		seguidor: "rubis",
		post:     postLari,
	}
	seguirMaria := &Seguir{
		seguidor: "rubis",
		post:     postMaria,
	}

	app.state.Update(seguirLari)
	app.state.Update(seguirMaria)

	// Executar servidor
	println("Servidor rodando em http://localhost:8000")
	err = app.RunWithTemplate(8000)
	if err != nil {
		panic(err)
	}
}
*/
