package main

import (
	"slices"
	"strings"
)

// checa se p.que esta no formato topico:descricao
func (v *VibeApp) Validar(p *Publicação) bool {
	topicos := strings.Split(p.Que, "\n")
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

type VibeApp struct {
	topicos         []string                     // topicos validos
	seguindo        map[string][]string          // usuario -> usuarios seguidos
	seguidores      map[string][]string          // usuario -> usuarios que seguem
	posts           map[string][]*Publicação     // usuario -> posts
	relacionamentos map[string][]*Relacionamento // usuario -> relacionamentos
}

func (v *VibeApp) Rank(usuario string) ([]*Publicação, []*Relacionamento) {
	seguidores, _ := v.seguidores[usuario]
	relacionamentos := make([]*Relacionamento, 0)
	for relSeguidoes := range seguidores {
		relacionamentos, _ := v.relacionamentos[relSeguidoes]
		for _, relacionamento := range relacionamentos {
			if relacionamento.Tipo() == "seguir" {
				seguindo = append(seguindo, relacionamento.Com())
			}
		}
	seguindo := v.seguindo[usuario]
	if len(seguindo) == 0 {
		return nil, seguidores
	}
	ultimos := make(map[string]*Publicação)

	for _, post := range v.posts {
		if slices.Contains(seguindo, post.Quem) {
			if ultimo, ok := ultimos[post.Quem]; !ok || post.Quando.After(ultimo.Quando) {
				ultimos[post.Quem] = post
			}
		}
	}
	ordemQualuer := make([]*Publicação, 0, len(ultimos))
	for _, post := range ultimos {
		ordemQualuer = append(ordemQualuer, post)
	}
	slices.SortFunc(ordemQualuer, func(a, b *Publicação) int {
		return a.Quando.Compare(b.Quando)
	})
	return ordemQualuer
}

type Seguir struct {
	seguidor string
	post     *Publicação
}

func (s *Seguir) Com() *Publicação {
	return s.post
}

func (s *Seguir) Tipo() string {
	return "seguir"
}

func (s *Seguir) Render() string {
	return "seguindo"
}

func (v *VibeApp) Update(interacao Interação) {
	seguir, ok := interacao.(*Seguir)
	if !ok {
		return
	}
	if seguir.post == nil {
		return
	}
	v.seguindo[seguir.seguidor] = append(v.seguindo[seguir.seguidor], seguir.post.Quem)
}

type VibeLayout struct {
	renderer Renderer
}

func (l *VibeLayout) Render(posts []*Publicação) string {
	html := "<div class='timeline'>"
	for _, post := range posts {
		html += l.renderer.Render(post, nil)
	}
	html += "</div>"
	print(html)
	return html
}
