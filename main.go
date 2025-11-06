package main

/*
import (
	"time"
)

type TimelineLayout struct {
	Renderer Renderer
}

func (l *TimelineLayout) Render(posts []*Publicação, interacoes []Interação) string {
	comInteracoes := make(map[*Publicação][]*Publicação)
	for _, post := range posts {
		if post.Interação != nil {
			comInteracoes[post] = append(comInteracoes[post], post)
		}
	}
	html := "<div class='timeline'>"
	for _, post := range posts {
		html += l.Renderer.Render(post, comInteracoes[post])
	}
	html += "</div>"
	return html
}

// signin, login, search, layoyt selection,post, interactions
func main() {
	vibeappp := App{
		renderer: nil,
		Layout: []Layout{
			&VibeLayout{
				renderer: &DefaultRenderer{},
			},
		},
		Repositório: &VibeApp{
			seguindo: make(map[string][]string),
			posts:    make([]*Publicação, 0),
			topicos:  []string{"lendo", "ouvindo", "namorando", "assistindo", "jogando", "preocupando", "comendo", "cobiçando"},
		},
	}

	postLari := Publicação{
		Quem:      "lari",
		Quando:    time.Now(),
		Onde:      "",
		Que:       "lendo:murder in mesopotamia\nouvindo:dupê\njogando:monstruosas",
		Interação: nil,
	}
	if !vibeappp.Repositório.Validar(&postLari) {
		panic("post invalido")
	}

	seguirRubis := &Seguir{
		seguidor: "rubis",
		post:     &postLari,
	}

	vibeappp.Repositório.Update(seguirRubis)

	postLari.Interação = &Seguir{
		seguidor: "lari",
		post:     &postLari,
	}

	vibeappp.Run(8000)
}
*/

func main() {

}
