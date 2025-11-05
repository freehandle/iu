# Template de Timeline - Vibe

Este é um template HTML/CSS/JavaScript moderno para uma timeline de posts com funcionalidades de interação e criação de novos posts.

## Características

### 🎨 Design Moderno
- Interface limpa e moderna com gradientes
- Design responsivo para desktop, tablet e mobile
- Animações suaves e transições elegantes
- Glassmorphism com backdrop-filter

### 📱 Layout Timeline
- Feed de posts em formato timeline vertical
- Linha temporal visual conectando os posts
- Área dedicada para criação de novos posts
- Sidebar com atividades recentes e trending topics

### ✨ Funcionalidades Interativas
- **Criação de Posts**: Área para escrever novos posts com sugestões de tópicos
- **Sistema de Tópicos**: Validação automática de tópicos (lendo:, ouvindo:, jogando:, etc.)
- **Interações**: Botões de like, comentário e compartilhamento
- **Filtros**: Filtros para visualizar todos os posts ou apenas seguindo
- **Notificações**: Sistema de notificações em tempo real

### 🎯 Compatibilidade com Go
O template foi projetado para ser compatível com a estrutura Go existente:
- Classes CSS que correspondem aos campos do struct `Post`
- Formatação de tempo compatível com `time.Time`
- Estrutura de interações que pode ser integrada com o sistema Go

## Estrutura de Arquivos

```
templates/
├── timeline.html      # Template HTML principal
├── timeline.css       # Estilos CSS
├── timeline.js        # Funcionalidades JavaScript
└── README.md         # Este arquivo
```

## Como Usar

### 1. Integração com Go
Para integrar com seu servidor Go, você pode:

```go
// No seu handler HTTP
func (a *App) Run(port int) {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        posts := a.state.Rank("rubis")
        
        // Carregar o template HTML
        tmpl, err := template.ParseFiles("templates/timeline.html")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        // Renderizar com os dados
        data := struct {
            Posts []*Post
        }{
            Posts: posts,
        }
        
        tmpl.Execute(w, data)
    })
    
    // Servir arquivos estáticos
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates"))))
    
    http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
```

### 2. Personalização
- **Cores**: Modifique as variáveis CSS no início do arquivo `timeline.css`
- **Tópicos**: Adicione ou remova tópicos no array `topics` no JavaScript
- **Layout**: Ajuste o grid layout no CSS para diferentes tamanhos de tela

### 3. Funcionalidades JavaScript
O JavaScript inclui:
- Auto-resize do textarea
- Validação de tópicos
- Sistema de notificações
- Interações com posts (like, comentário, compartilhamento)
- Filtros de timeline
- Animações suaves

## Tópicos Suportados

O sistema suporta os seguintes tópicos (baseado no código Go):
- `lendo:` - Para livros, artigos, etc.
- `ouvindo:` - Para música, podcasts, etc.
- `jogando:` - Para jogos
- `assistindo:` - Para filmes, séries, etc.
- `comendo:` - Para refeições
- `preocupando:` - Para preocupações
- `namorando:` - Para relacionamentos
- `cobiçando:` - Para desejos

## Responsividade

O template é totalmente responsivo:
- **Desktop**: Layout em grid com sidebar
- **Tablet**: Sidebar se move para cima
- **Mobile**: Layout em coluna única, otimizado para toque

## Navegadores Suportados

- Chrome 80+
- Firefox 75+
- Safari 13+
- Edge 80+

## Próximos Passos

Para integrar completamente com seu sistema Go:

1. **API Endpoints**: Crie endpoints para criar posts e interações
2. **WebSocket**: Implemente atualizações em tempo real
3. **Autenticação**: Adicione sistema de login/logout
4. **Persistência**: Conecte com banco de dados
5. **Upload de Mídia**: Adicione suporte para imagens e vídeos

## Licença

Este template é fornecido como exemplo e pode ser modificado conforme necessário para seu projeto. 