package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Follow struct {
	seguidor string
	seguido  string
}

type Post struct {
	quem      string
	quando    time.Time
	onde      string
	que       string
	interação Interaction
}

type State interface {
	Update(Interaction)
	Rank(usuario string) []*Post
	Validar(p *Post) bool
}

type Interaction interface {
	Com() *Post
	Tipo() string
}

type Renderer interface {
	Render(*Post, []Interaction) string //HTML
}

type DefaultRenderer struct {
	style string
}

func (r *DefaultRenderer) Render(p *Post, interacoes []Interaction) string {
	html := "<div class='post'>"
	html += fmt.Sprintf("<div class='quem'>%s</div>", p.quem)
	html += fmt.Sprintf("<div class='quando'>%s</div>", p.quando.Format(time.RFC3339))
	html += fmt.Sprintf("<div class='onde'>%s</div>", p.onde)
	linhas := strings.Split(p.que, "\n")
	html_linhas := ""
	for _, linha := range linhas {
		html_linhas += fmt.Sprintf("<div class='linha'>%s</div>", linha)
	}
	html += fmt.Sprintf("<div class='que'>%s</div>", html_linhas)
	html += "</div>"
	return html
}

type Layout interface {
	Render([]*Post) string
}

type TimelineLayout struct {
	renderer Renderer
}

// func (l *TimelineLayout) Render(posts []*Post) string {
// 	comInteracoes := make(map[*Post][]Interaction)
// 	for _, post := range posts {
// 		if post.interação != nil {
// 			comInteracoes[post] = append(comInteracoes[post], post.interação)
// 		}
// 	}
// 	html := "<div class='timeline'>"
// 	for _, post := range posts {
// 		html += l.renderer.Render(post, post.interação)
// 	}
// 	html += "</div>"
// 	return html
// }

func (l *TimelineLayout) Render(posts []*Post) string {
	return ""
}

type App struct {
	renderer Renderer
	layout   []Layout
	state    State
}

func (a *App) Run(port int) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		posts := a.state.Rank("rubis")
		for _, post := range posts {
			fmt.Printf("%p\n", *post)
		}
		if len(posts) == 0 {
			w.Write([]byte("Nenhum post encontrado"))
		} else {
			w.Write([]byte(a.layout[0].Render(posts)))
		}
	})
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

}

// signin, login, search, layoyt selection,post, interactions
func main() {
	vibeappp := App{
		renderer: nil,
		layout: []Layout{
			&VibeLayout{
				renderer: &DefaultRenderer{},
			},
		},
		state: &VibeState{
			seguindo: make(map[string][]string),
			posts:    make([]*Post, 0),
			topicos:  []string{"lendo", "ouvindo", "namorando", "assistindo", "jogando", "preocupando", "comendo", "cobiçando"},
		},
	}

	postLari := Post{
		quem:      "lari",
		quando:    time.Now(),
		onde:      "",
		que:       "lendo:murder in mesopotamia\nouvindo:dupê\njogando:monstruosas",
		interação: nil,
	}
	if !vibeappp.state.Validar(&postLari) {
		panic("post invalido")
	}

	seguirRubis := &Seguir{
		seguidor: "rubis",
		post:     &postLari,
	}

	vibeappp.state.Update(seguirRubis)

	postLari.interação = &Seguir{
		seguidor: "lari",
		post:     &postLari,
	}

	vibeappp.Run(8000)
}
