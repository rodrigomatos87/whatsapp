package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// Mídia (imagem/PDF) enviada ao copiloto: guardada aqui por até UM DIA e
// apenas de remetentes AUTORIZADOS (a ponte confirma antes do download —
// mídia de grupo, lista ou desconhecido nunca toca o disco).
const dirMidiaPonte = "/opt/Ravi/ia/whats_midia"
const midiaMaxBytes = 25 * 1024 * 1024
const midiaRetencao = 24 * time.Hour

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

// limparMidiasPonte apaga da pasta de mídia tudo o que passou da retenção.
// Oportunista (roda a cada mídia nova); o cron diário do PHP é o reforço.
func limparMidiasPonte() {
	entradas, err := os.ReadDir(dirMidiaPonte)
	if err != nil {
		return
	}
	for _, e := range entradas {
		if e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > midiaRetencao {
			os.Remove(filepath.Join(dirMidiaPonte, e.Name()))
		}
	}
}

// consultarPonte pergunta ao PHP se o remetente é um usuário liberado NAQUELA
// conta (op=consulta). É o gate que impede baixar mídia de quem não deve.
func (r *repository) consultarPonte(token, conta, numero string) bool {
	resp, err := clientePonte.PostForm(r.ponteURL, url.Values{
		"token":     {token},
		"op":        {"consulta"},
		"conta":     {conta},
		"remetente": {numero},
	})
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var out struct {
		Ok         bool `json:"ok"`
		Autorizado bool `json:"autorizado"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Ok && out.Autorizado
}

func extensaoMidia(mime string) string {
	switch {
	case strings.Contains(mime, "pdf"):
		return ".pdf"
	case strings.Contains(mime, "png"):
		return ".png"
	case strings.Contains(mime, "webp"):
		return ".webp"
	case strings.Contains(mime, "gif"):
		return ".gif"
	default:
		return ".jpg"
	}
}

func (r *repository) encaminharPonte(cli *whatsmeow.Client, evt *events.Message) {
	if r.ponteURL == "" {
		return
	}
	if evt.Info.IsFromMe {
		return
	}
	chat := evt.Info.Chat
	// Só conversa direta (1:1): grupo, lista, status e canal ficam de fora —
	// nem texto, nem (principalmente) mídia.
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

	// Mídia aceita: imagem ou documento PDF (com a legenda como texto).
	img := evt.Message.GetImageMessage()
	doc := evt.Message.GetDocumentMessage()
	var baixavel whatsmeow.DownloadableMessage
	midiaTipo, midiaNome, midiaMime := "", "", ""
	var midiaTam uint64
	if img != nil {
		baixavel = img
		midiaTipo = "imagem"
		midiaMime = img.GetMimetype()
		midiaNome = "imagem" + extensaoMidia(midiaMime)
		midiaTam = img.GetFileLength()
		if texto == "" {
			texto = img.GetCaption()
		}
	} else if doc != nil && strings.Contains(strings.ToLower(doc.GetMimetype()), "pdf") {
		baixavel = doc
		midiaTipo = "pdf"
		midiaMime = doc.GetMimetype()
		midiaNome = doc.GetFileName()
		if midiaNome == "" {
			midiaNome = "documento.pdf"
		}
		midiaTam = doc.GetFileLength()
		if texto == "" {
			texto = doc.GetCaption()
		}
	}

	texto = strings.TrimSpace(texto)
	if texto == "" && baixavel == nil {
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

	// Download SÓ depois de o PHP confirmar que o remetente é um usuário
	// liberado nesta conta; mídia de mais ninguém é gravada em disco.
	campos := url.Values{
		"token":     {token},
		"conta":     {conta},
		"remetente": {numero},
		"chat":      {chat.String()},
		"nome":      {evt.Info.PushName},
		"texto":     {texto},
	}
	if baixavel != nil {
		if midiaTam > midiaMaxBytes {
			log.Warnf("ponte: mídia de %s ignorada (%.1f MB passa do teto)", numero, float64(midiaTam)/1048576)
			baixavel = nil
		} else if !r.consultarPonte(token, conta, numero) {
			baixavel = nil
		}
	}
	if baixavel != nil {
		limparMidiasPonte()
		dados, err := cli.Download(context.Background(), baixavel)
		if err != nil || len(dados) == 0 {
			log.Warnf("ponte: falha ao baixar mídia de %s: %v", numero, err)
		} else if len(dados) > midiaMaxBytes {
			log.Warnf("ponte: mídia de %s ignorada (%.1f MB passa do teto)", numero, float64(len(dados))/1048576)
		} else {
			if err := os.MkdirAll(dirMidiaPonte, 0777); err == nil {
				os.Chmod(dirMidiaPonte, 0777) // o PHP (www-data) também apaga na retenção
				suf := make([]byte, 4)
				rand.Read(suf)
				arq := fmt.Sprintf("%d_%s_%s_%s%s", time.Now().Unix(), conta, numero, hex.EncodeToString(suf), extensaoMidia(midiaMime))
				caminho := filepath.Join(dirMidiaPonte, arq)
				if err := os.WriteFile(caminho, dados, 0644); err == nil {
					campos.Set("midia_caminho", caminho)
					campos.Set("midia_tipo", midiaTipo)
					campos.Set("midia_nome", midiaNome)
					campos.Set("midia_bytes", fmt.Sprintf("%d", len(dados)))
				} else {
					log.Warnf("ponte: falha ao gravar mídia: %v", err)
				}
			}
		}
	}

	if campos.Get("texto") == "" && campos.Get("midia_caminho") == "" {
		return
	}

	resp, err := clientePonte.PostForm(r.ponteURL, campos)
	if err != nil {
		log.Warnf("ponte do copiloto indisponível: %v", err)
		return
	}
	resp.Body.Close()
}
