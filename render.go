package main

import (
	"fmt"
	"strings"
	"time"
)

var publicacaoTemplate = `
<div class="post">
	<div class="post-header">
		<div class="quem">lari</div>
		<div class="quando">5 min atrás</div>
	</div>
	<div class="que">
		<div class="linha"><strong>lendo:</strong> murder in mesopotamia</div>
		<div class="linha"><strong>ouvindo:</strong> dupê</div>
		<div class="linha"><strong>jogando:</strong> monstruosas</div>
	</div>
	<div class="post-actions">
		<button class="action-btn" onclick="likePost(this)">
			<i class="far fa-heart"></i>
			<span>3</span>
		</button>
		<button class="action-btn" onclick="commentPost(this)">
			<i class="far fa-comment"></i>
			<span>1</span>
		</button>
		<button class="action-btn" onclick="sharePost(this)">
			<i class="far fa-share-square"></i>
		</button>
	</div>
`

type DefaultRenderer struct{}

func generalPostRenderer(quem, quando, onde, que string) string {
	html := "<div class='post'><div class='post-header'>"
	html += fmt.Sprintf("<div class='quem'>%s</div>", quem)
	html += fmt.Sprintf("<div class='quando'>%s</div></div>", quando)
	html += fmt.Sprintf("<div class='onde'>%s</div>", onde)
	html += "</div><div class='que'>"
	linhas := strings.Split(que, "\n")
	html_linhas := ""
	for _, linha := range linhas {
		html_linhas += fmt.Sprintf("<div class='linha'>%s</div>", linha)

	}
	html += "div"

	html += fmt.Sprintf("<div class='que'>%s</div>", que)
	return html
}

func (r *DefaultRenderer) Render(p *Publicação, interacoes []*Publicação) string {
	html := "<div class='post'>"
	html += generalPostRenderer(p.Quem, p.Quando.Format(time.RFC3339), p.Onde, p.Que)
	html += "<div class='interacoes'>"
	for _, subpost := range interacoes {
		if subpost == nil || subpost.Interação == nil || !subpost.Interação.Visible() {
			continue
		}
		html_sub := generalPostRenderer(subpost.Interação.Com().Quem, subpost.Interação.Com().Quando.Format(time.RFC3339), subpost.Interação.Com().Onde, subpost.Interação.Com().Que)
		html += fmt.Sprintf("<div class='interacao'>%s</div>", html_sub)
	}
	html += "</div>"
	html += "</div>"
	return html
}
