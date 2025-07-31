package main

import (
	"slices"
	"strings"
)

// checa se p.que esta no formato topico:descricao
func (v *VibeState) Validar(p *Post) bool {
	topicos := strings.Split(p.que, "\n")
	for _, topico := range topicos {
		quebra := strings.Split(topico, ":")
		if !slices.Contains(v.topicos, quebra[0]) {
			return false
		}
	}
	if len(topicos) > 0 {
		v.posts = append(v.posts, p)
		return true
	}
	return false
}

type VibeState struct {
	topicos  []string
	seguindo map[string][]string
	posts    []*Post
}

func (v *VibeState) Rank(usuario string) []*Post {
	seguindo := v.seguindo[usuario]
	if len(seguindo) == 0 {
		return nil
	}
	ultimos := make(map[string]*Post)

	for _, post := range v.posts {
		if slices.Contains(seguindo, post.quem) {
			if ultimo, ok := ultimos[post.quem]; !ok || post.quando.After(ultimo.quando) {
				ultimos[post.quem] = post
			}
		}
	}
	ordemQualuer := make([]*Post, 0, len(ultimos))
	for _, post := range ultimos {
		ordemQualuer = append(ordemQualuer, post)
	}
	slices.SortFunc(ordemQualuer, func(a, b *Post) int {
		return a.quando.Compare(b.quando)
	})
	return ordemQualuer
}

type Seguir struct {
	seguidor string
	post     *Post
}

func (s *Seguir) Com() *Post {
	return s.post
}

func (s *Seguir) Tipo() string {
	return "seguir"
}

func (v *VibeState) Update(interacao Interaction) {
	seguir, ok := interacao.(*Seguir)
	if !ok {
		return
	}
	if seguir.post == nil {
		return
	}
	v.seguindo[seguir.seguidor] = append(v.seguindo[seguir.seguidor], seguir.post.quem)
}

type VibeLayout struct {
	renderer Renderer
}

func (l *VibeLayout) Render(posts []*Post) string {
	html := "<div class='timeline'>"
	for _, post := range posts {
		html += l.renderer.Render(post, nil)
	}
	html += "</div>"
	print(html)
	return html
}
