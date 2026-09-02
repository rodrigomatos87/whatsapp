package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"ravi/models"
	"strings"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// clientePronto resolve a conta e garante que ela está autenticada e
// conectada — o pré-requisito de toda operação de verdade.
func (r *repository) clientePronto(ctx context.Context, conta string) (*whatsmeow.Client, error) {
	status, err := r.Status(ctx, conta)
	if err != nil {
		return nil, err
	}

	if err := status.IsOK(); err != nil {
		return nil, err
	}

	sess, err := r.sessao(conta)
	if err != nil {
		return nil, err
	}

	return sess.Client()
}

func (r *repository) GetGroups(ctx context.Context, conta string) ([]*types.GroupInfo, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return nil, err
	}

	return conn.GetJoinedGroups(context.Background())
}

func (r *repository) GetContacts(ctx context.Context, conta string) (map[types.JID]types.ContactInfo, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return nil, err
	}

	raw, err := conn.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil, err
	}

	// Com o LID addressing do WhatsApp, a maioria dos contatos passou a vir
	// chaveada por LID (@lid) e sem o número real (só o RedactedPhone, ex.:
	// "+55∙∙∙∙∙∙∙∙28"). Aqui resolvemos o LID para o número (PN) via o mapa do
	// store e rechaveamos o contato por @s.whatsapp.net, para a UI exibir o
	// número de verdade. Quando o mesmo contato aparece como LID e como PN,
	// os dados são mesclados para não perder nome.
	out := make(map[types.JID]types.ContactInfo, len(raw))
	for jid, info := range raw {
		key := jid
		if jid.Server == types.HiddenUserServer {
			if pn, err := conn.Store.LIDs.GetPNForLID(context.Background(), jid); err == nil && pn.User != "" {
				key = pn
			}
		}
		if existing, ok := out[key]; ok {
			out[key] = mergeContact(existing, info)
		} else {
			out[key] = info
		}
	}

	return out, nil
}

// mergeContact preenche campos vazios de "a" com os de "b" (usado quando o
// mesmo contato chega chaveado por LID e por número).
func mergeContact(a, b types.ContactInfo) types.ContactInfo {
	if a.FirstName == "" {
		a.FirstName = b.FirstName
	}
	if a.FullName == "" {
		a.FullName = b.FullName
	}
	if a.PushName == "" {
		a.PushName = b.PushName
	}
	if a.BusinessName == "" {
		a.BusinessName = b.BusinessName
	}
	if a.RedactedPhone == "" {
		a.RedactedPhone = b.RedactedPhone
	}
	a.Found = a.Found || b.Found
	return a
}

// CheckNumber verifica se um número está no WhatsApp e devolve o JID canônico
// (já normalizado pelo WhatsApp). phone deve vir em formato internacional com
// "+" (ex.: +5544999990000).
func (r *repository) CheckNumber(ctx context.Context, conta, phone string) (models.NumberCheck, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return models.NumberCheck{}, err
	}

	resp, err := conn.IsOnWhatsApp(context.Background(), []string{phone})
	if err != nil {
		return models.NumberCheck{}, err
	}

	if len(resp) == 0 {
		return models.NumberCheck{Query: phone, OnWhatsApp: false}, nil
	}

	r0 := resp[0]
	return models.NumberCheck{
		Query:      r0.Query,
		JID:        r0.JID.String(),
		OnWhatsApp: r0.IsIn,
	}, nil
}

// Logout desconecta e APAGA a conta do store. Só a sessão pedida cai; as
// outras contas seguem no ar (por isso não há mais os.Exit aqui). Se a conta
// era a principal, a próxima da lista assume.
func (r *repository) Logout(ctx context.Context, conta string) (err error) {
	sess, err := r.sessao(conta)
	if err != nil {
		return err
	}

	conn, err := sess.Client()
	if err != nil {
		return err
	}

	numero := ""
	if conn.Store.ID != nil {
		numero = conn.Store.ID.User
	}

	conn.Logout(context.Background())
	conn.Disconnect()
	conn.Store.Delete(context.Background())

	sess.SetClient(nil)
	sess.SetQrCode("")

	r.l.Lock()
	defer r.l.Unlock()
	if numero != "" {
		delete(r.sessoes, numero)
	}
	if r.principal == numero {
		r.principal = ""
		if nums := r.contasOrdenadas(); len(nums) > 0 {
			r.principal = nums[0]
		}
		r.salvarPrincipal()
	}

	return nil
}

func (r *repository) GroupInfoByLink(ctx context.Context, conta, link string) (*types.GroupInfo, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return nil, err
	}

	resp, err := conn.GetGroupInfoFromLink(context.Background(), link)
	if err != nil {
		return nil, fmt.Errorf("erro ao resolver o link do convite do grupo: %v", err)
	}

	return resp, nil
}

func (r *repository) DeviceInfo(ctx context.Context, conta string) (*types.JID, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return nil, err
	}

	return conn.Store.ID, nil
}

func (r *repository) Status(ctx context.Context, conta string) (models.WhatsAppAPIStatus, error) {
	sess, err := r.sessao(conta)
	if err != nil {
		// Sem nenhuma conta pareada o status legado sempre foi "offline",
		// nunca erro — o front usa isso para exibir o fluxo de QR.
		if conta == "" {
			return models.WhatsAppAPIStatus{}.IsOffline(), nil
		}
		return models.WhatsAppAPIStatus{}, err
	}

	conn, err := sess.Client()
	if err != nil {
		return models.WhatsAppAPIStatus{}, err
	}

	if conn.IsConnected() && conn.Store.ID != nil {
		return models.WhatsAppAPIStatus{}.IsOnline(), nil
	}

	if conn.IsConnected() {
		return models.WhatsAppAPIStatus{}.IsConnected(), nil
	}

	return models.WhatsAppAPIStatus{}.IsOffline(), nil
}

// ListarContas devolve todas as contas pareadas e se há pareamento em curso.
func (r *repository) ListarContas(ctx context.Context) ([]models.Conta, bool, error) {
	r.l.RLock()
	defer r.l.RUnlock()

	contas := []models.Conta{}
	for _, numero := range r.contasOrdenadas() {
		sess := r.sessoes[numero]
		conta := models.Conta{
			Numero:    numero,
			Principal: numero == r.principal,
		}
		if cli, err := sess.Client(); err == nil {
			if cli.Store.ID != nil {
				conta.JID = cli.Store.ID.String()
			}
			conta.Nome = cli.Store.PushName
			conta.Conectado = cli.IsConnected() && cli.Store.ID != nil
		}
		contas = append(contas, conta)
	}

	pareando := false
	if r.pareando != nil {
		if _, err := r.pareando.Client(); err == nil {
			pareando = true
		}
	}

	return contas, pareando, nil
}

func (r *repository) Send(ctx context.Context, conta, jid1, text string) (string, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	jid, err := parseJID(jid1)
	if err != nil {
		return "", err
	}

	message := &waProto.Message{Conversation: proto.String(text)}

	if _, err := conn.SendMessage(ctx, jid, message); err != nil {
		return "", err
	}

	return "mensagem enviada com sucesso!", nil
}

// EnviarImagem sobe uma imagem LOCAL (só da pasta de mídia do copiloto, que
// tem retenção de um dia) e a envia com legenda. É o caminho pelo qual o
// copiloto manda gráficos gerados no servidor para o WhatsApp do usuário.
func (r *repository) EnviarImagem(ctx context.Context, conta, jid1, caminho, legenda string) (string, error) {
	conn, err := r.clientePronto(ctx, conta)
	if err != nil {
		return "", err
	}

	limpo := filepath.Clean(caminho)
	if !strings.HasPrefix(limpo, "/opt/Ravi/ia/whats_midia/") {
		return "", fmt.Errorf("imagem fora da pasta de mídia do copiloto")
	}
	dados, err := os.ReadFile(limpo)
	if err != nil {
		return "", fmt.Errorf("erro ao ler a imagem: %v", err)
	}
	if len(dados) == 0 || len(dados) > 10*1024*1024 {
		return "", fmt.Errorf("imagem vazia ou maior que 10 MB")
	}

	mime := "image/png"
	switch strings.ToLower(filepath.Ext(limpo)) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".webp":
		mime = "image/webp"
	case ".gif":
		mime = "image/gif"
	}

	jid, err := parseJID(jid1)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	up, err := conn.Upload(ctx, dados, whatsmeow.MediaImage)
	if err != nil {
		return "", fmt.Errorf("erro no upload da imagem: %v", err)
	}

	msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		Mimetype:      proto.String(mime),
		Caption:       proto.String(legenda),
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}

	if _, err := conn.SendMessage(ctx, jid, msg); err != nil {
		return "", err
	}

	return "imagem enviada com sucesso!", nil
}

func parseJID(arg string) (types.JID, error) {
	if arg[0] == '+' {
		arg = arg[1:]
	}

	if !strings.ContainsRune(arg, '@') {
		return types.NewJID(arg, types.DefaultUserServer), nil
	} else {
		recipient, err := types.ParseJID(arg)
		if err != nil {
			return recipient, fmt.Errorf("invalid JID %s: %v", arg, err)

		} else if recipient.User == "" {
			return recipient, fmt.Errorf("invalid JID %s: no server specified", arg)
		}
		return recipient, nil
	}
}
