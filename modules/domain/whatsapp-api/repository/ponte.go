package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Ponte do Copiloto: cada mensagem RECEBIDA numa conversa direta é entregue ao
// PHP (ia/whats_ponte.php), que decide se o remetente é um usuário do sistema
// com o copiloto liberado no perfil — e, se for, responde pelo próprio
// WhatsApp. O serviço só entrega; quem autentica, autoriza e responde é o PHP.

// O segredo compartilhado com o PHP vive ao lado do store (mdtest.db). O
// serviço o cria no primeiro uso; o PHP recusa qualquer chamada sem ele
// (defesa extra além do loopback).
const arquivoTokenPonte = "ponte.token"

var clientePonte = &http.Client{Timeout: 15 * time.Second}

func tokenPonte() string {
	if b, err := os.ReadFile(arquivoTokenPonte); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	t := hex.EncodeToString(b)
	if err := os.WriteFile(arquivoTokenPonte, []byte(t), 0644); err != nil {
		log.Errorf("erro ao gravar o token da ponte do copiloto: %v", err)
		return ""
	}
	return t
}

func (r *repository) encaminharPonte(cli *whatsmeow.Client, evt *events.Message) {
	if r.ponteURL == "" {
		return
	}
	if evt.Info.IsFromMe {
		return
	}
	chat := evt.Info.Chat
	// Só conversa direta (1:1): grupo, lista, status e canal ficam de fora.
	if chat.Server == types.GroupServer || chat.Server == types.BroadcastServer || chat.Server == types.NewsletterServer {
		return
	}
	// Mensagens antigas reentregues numa reconexão não devem acordar o copiloto.
	if !evt.Info.Timestamp.IsZero() && time.Since(evt.Info.Timestamp) > 5*time.Minute {
		return
	}

	texto := evt.Message.GetConversation()
	if texto == "" && evt.Message.GetExtendedTextMessage() != nil {
		texto = evt.Message.GetExtendedTextMessage().GetText()
	}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return
	}
	if len(texto) > 8000 {
		texto = texto[:8000]
	}

	// Número real (PN) do remetente: com o LID addressing o Sender pode vir
	// @lid; o SenderAlt e o mapa LID→PN do store resolvem para o telefone.
	numero := ""
	if evt.Info.Sender.Server == types.DefaultUserServer {
		numero = evt.Info.Sender.User
	}
	if numero == "" && evt.Info.SenderAlt.Server == types.DefaultUserServer {
		numero = evt.Info.SenderAlt.User
	}
	if numero == "" && evt.Info.Sender.Server == types.HiddenUserServer {
		if pn, err := cli.Store.LIDs.GetPNForLID(context.Background(), evt.Info.Sender); err == nil && pn.User != "" {
			numero = pn.User
		}
	}
	if numero == "" {
		return
	}

	conta := ""
	if cli.Store.ID != nil {
		conta = cli.Store.ID.User
	}
	if conta == "" {
		return
	}

	token := tokenPonte()
	if token == "" {
		return
	}

	resp, err := clientePonte.PostForm(r.ponteURL, url.Values{
		"token":     {token},
		"conta":     {conta},
		"remetente": {numero},
		"chat":      {chat.String()},
		"nome":      {evt.Info.PushName},
		"texto":     {texto},
	})
	if err != nil {
		log.Warnf("ponte do copiloto indisponível: %v", err)
		return
	}
	resp.Body.Close()
}
