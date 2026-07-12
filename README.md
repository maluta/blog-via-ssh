# Blog via SSH

Um blog para terminal escrito em Go. Em vez de abrir uma página convencional,
o visitante pode se conectar por SSH e navegar por uma interface TUI com lista
de posts, páginas de apresentação e contatos. O projeto também inicia uma
landing page web simples para apresentar o blog e mostrar como acessá-lo.

Ao conectar, o visitante não recebe um shell no servidor: o SSH inicia somente
a aplicação do blog.

<img width="670" height="506" alt="image" src="https://github.com/user-attachments/assets/84aa0937-5211-41d2-88e7-efd206f12ad6" />


## Stack

- [Go](https://go.dev/) — servidor e aplicação.
- [Wish](https://github.com/charmbracelet/wish) — servidor SSH.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) e
  [Bubbles](https://github.com/charmbracelet/bubbles) — interface TUI e seus
  componentes.
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — estilos no terminal.
- [Glamour](https://github.com/charmbracelet/glamour) — renderização dos posts
  em Markdown.

## Funcionalidades

- Blog navegável diretamente pelo cliente `ssh`.
- Abas de posts, apresentação e contato.
- Posts em arquivos Markdown com frontmatter YAML.
- Renderização de Markdown e destaque de sintaxe no terminal.
- Recarregamento dos posts sem reiniciar o servidor.
- Landing page web executada junto com o servidor SSH.
- Geração automática e persistente da host key do servidor.

## Pré-requisitos

- Go na versão indicada em [`go.mod`](go.mod), ou uma versão compatível.
- Um cliente SSH para testar a interface no terminal.

## Executando localmente

Clone o repositório e entre no diretório do projeto:

```bash
git clone <URL_DO_REPOSITORIO>
cd <DIRETORIO_DO_REPOSITORIO>
```

Baixe as dependências e inicie a aplicação:

```bash
go mod download
go run .
```

Na primeira execução, o servidor cria a host key `.ssh/id_ed25519`. Essa chave
identifica o servidor SSH, deve permanecer privada e já está excluída do Git.

Com a aplicação em execução, abra outro terminal e conecte-se à TUI:

```bash
ssh -p 23234 localhost
```

A landing page estará disponível em:

```text
http://localhost:8080
```

Para encerrar a aplicação, pressione `Ctrl+C` no terminal em que ela está
rodando.

> Se a host key local for apagada e recriada, o cliente SSH pode avisar que a
> identificação do servidor mudou. Remova somente a entrada local de teste com
> `ssh-keygen -R '[localhost]:23234'` antes de conectar novamente.

## Personalizando o conteúdo

Antes de publicar seu próprio blog, altere **`content.go`**. Esse arquivo contém
as partes estáticas e autorais da interface, incluindo:

- banner em ASCII;
- frase de apresentação;
- texto da aba “Sobre”;
- links e dados da aba “Contato”.

Revise todos esses valores e substitua o conteúdo de exemplo pelos seus dados.
A landing page web reutiliza essas informações, portanto a personalização é
refletida tanto no terminal quanto no navegador.

## Escrevendo posts

Os posts ficam em `posts/` como arquivos `.md`. Cada arquivo deve começar com
um frontmatter YAML:

~~~markdown
---
title: "Título do post"
date: 2026-01-15
tags: [go, terminal]
---

Conteúdo do post em **Markdown**.

```go
package main

func main() {}
```
~~~

É recomendável usar nomes como `2026-01-15-titulo-do-post.md`. Os posts são
ordenados da data mais recente para a mais antiga. Um arquivo inválido é
ignorado e registrado no log, sem derrubar a aplicação.

Novos arquivos aparecem nas próximas conexões. Em uma sessão já aberta,
pressione `r` no índice do blog para recarregar a lista.

Se o título contiver caracteres com significado especial em YAML, como `:`,
`#`, `[` ou `{`, coloque-o entre aspas.

## Controles da interface

- `Tab` ou `1`–`3`: alternar entre as abas.
- `↑` / `↓`: navegar pela lista ou pelo conteúdo.
- `Enter`: abrir o post selecionado.
- `PgUp` / `PgDn`: percorrer um post longo.
- `Esc`: voltar do post para a lista.
- `r`: recarregar os posts.
- `q`: encerrar a sessão.

## Configuração

O servidor aceita as seguintes variáveis de ambiente:

| Variável | Padrão | Finalidade |
| --- | --- | --- |
| `HOST` | `0.0.0.0` | Endereço em que o servidor SSH escuta. |
| `PORT` | `23234` | Porta do servidor SSH. |
| `WEB_DOMAIN` | vazio | Domínio público usado para ativar HTTPS em produção. |

Exemplo usando outra porta SSH:

```bash
HOST=0.0.0.0 PORT=2222 go run .
```

Sem `WEB_DOMAIN`, a landing page usa HTTP na porta `8080`, adequado para
desenvolvimento local. Com `WEB_DOMAIN` definido, a aplicação tenta atender o
domínio com HTTPS e certificado automático, além do redirecionamento HTTP.

## Gerando um binário

Para compilar para a máquina atual:

```bash
go build -o blog-ssh .
```

Exemplo de compilação estática para Linux AMD64:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o blog-ssh .
```

O diretório `posts/` não é incorporado ao binário. Ao distribuir a aplicação,
copie também os posts e execute o processo a partir de um diretório de trabalho
que contenha essa pasta.

## Checklist de deploy

O projeto pode ser hospedado em qualquer servidor Linux que aceite conexões nas
portas escolhidas. Uma implantação típica deve:

1. Compilar o binário para a arquitetura do servidor.
2. Copiar o binário e o diretório `posts/` para um diretório próprio.
3. Criar um usuário de serviço sem privilégios e dar a ele acesso somente aos
   arquivos necessários.
4. Executar o binário com um supervisor, como systemd, configurando o diretório
   de trabalho e as variáveis de ambiente.
5. Manter `.ssh/id_ed25519` em armazenamento persistente, com permissões
   restritas, para que a identidade SSH não mude a cada atualização.
6. Liberar no firewall somente as portas necessárias e configurar DNS para o
   endereço público do servidor.
7. Se usar `WEB_DOMAIN`, permitir que a aplicação atenda HTTP/HTTPS e confirme
   que o domínio resolve corretamente antes de solicitar o certificado.
8. Preservar uma porta separada para o SSH administrativo antes de colocar o
   blog na porta `22`.

Portas abaixo de `1024` normalmente exigem privilégios adicionais. Evite
executar a aplicação inteira como `root`; prefira conceder somente a capacidade
de bind necessária ao binário ou encaminhar portas com um proxy/firewall.

> Atenção: mover o SSH administrativo para outra porta é uma operação que pode
> bloquear o acesso ao servidor. Teste uma segunda sessão antes de fechar a
> conexão atual e mantenha um meio alternativo de recuperação.

## Organização do projeto

- `content.go`: banner, apresentação, biografia e contatos personalizáveis.
- `posts.go`: leitura, validação, ordenação e renderização dos posts.
- `ui.go`: estado, navegação e visual da interface TUI.
- `main.go`: configuração e inicialização do servidor SSH.
- `web.go`: landing page HTTP/HTTPS.
- `posts/`: conteúdo Markdown carregado em tempo de execução.
- `snapshot_test.go`: testes da interface e da renderização.

## Testes

Execute a suíte com:

```bash
go test ./...
```
