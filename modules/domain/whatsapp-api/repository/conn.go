package repository

import (
	"context"
	"fmt"
	whatsappapi "ravi/modules/domain/whatsapp-api"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	meowWaProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var log waLog.Logger

// client é UMA sessão de WhatsApp (uma conta pareada, ou o slot de
// pareamento de uma conta nova enquanto o QR não é lido).
type client struct {
	_cli        *whatsmeow.Client
	_lastReq    time.Time
	_qrcode     string
	l           sync.RWMutex
	_qrCodeLock uint32
}

func (c *client) SetClient(cli *whatsmeow.Client) {
	c.l.Lock()
	c._cli = cli
	c.l.Unlock()
}

func (c *client) Client() (*whatsmeow.Client, error) {
	c.l.Lock()
	defer c.l.Unlock()
	if c._cli == nil {
		return nil, fmt.Errorf("whatsApp API não está pronta")
	}

	return c._cli, nil
}

func (c *client) SetQrCode(s string) {
	c.l.Lock()
	c._qrcode = s
	c.l.Unlock()
}

func (c *client) getQrCode() string {
	c.l.Lock()
	defer c.l.Unlock()
	return c._qrcode
}

// repository gerencia N sessões (multi-conta). A chave do mapa é o número
// da conta (JID.User). "principal" é a conta usada pelas rotas legadas
// (/send, /check-status, ...) — é a herança dos ambientes do Ravi.
type repository struct {
	_logLevel string
	_storage  *sqlstore.Container
	l         sync.RWMutex
	sessoes   map[string]*client
	pareando  *client
	principal string
	ponteURL  string
}

func New(logLevel, dbDialect, dbAddress string, requestFullSync bool, ponteURL string) (whatsappapi.Repository, error) {
	waBinary.IndentXML = true
	store.DeviceProps.RequireFullSync = proto.Bool(false)
	store.DeviceProps.PlatformType = meowWaProto.DeviceProps_FIREFOX.Enum()
	store.SetOSInfo("Ravi Monitor", store.GetWAVersion())
	log = waLog.Stdout("Handler", logLevel, true)

	if requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
	}
	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(context.Background(), dbDialect, dbAddress, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	repo := &repository{
		_logLevel: logLevel,
		_storage:  storeContainer,
		sessoes:   map[string]*client{},
		ponteURL:  strings.TrimSpace(ponteURL),
	}

	// Sobe uma sessão para CADA device pareado no store (multi-conta).
	// Erro de conexão de uma conta não derruba o serviço: as demais rotas
	// (e o pareamento de contas novas) precisam continuar respondendo.
	devices, err := storeContainer.GetAllDevices(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %v", err)
	}

	for _, device := range devices {
		if device.ID == nil {
			continue
		}
		cli := repo.novoCliente(device)
		sess := &client{}
		sess.SetClient(cli)
		repo.sessoes[device.ID.User] = sess
		if err := cli.Connect(); err != nil {
			log.Errorf("erro ao conectar a conta %s: %v", device.ID.User, err)
		}
	}

	repo.carregarPrincipal()

	log.Infof("API iniciada com %d conta(s); principal: %q", len(repo.sessoes), repo.principal)
	return repo, nil
}

// sessao resolve a conta pedida; conta vazia = principal (herança dos
// ambientes e das rotas legadas).
func (r *repository) sessao(conta string) (*client, error) {
	r.l.RLock()
	defer r.l.RUnlock()
	if conta == "" {
		conta = r.principal
	}
	if conta == "" {
		return nil, fmt.Errorf("nenhuma conta de WhatsApp conectada")
	}
	s, ok := r.sessoes[conta]
	if !ok {
		return nil, fmt.Errorf("a conta %s não está conectada neste servidor", conta)
	}
	return s, nil
}

// contasOrdenadas devolve os números das contas em ordem estável.
func (r *repository) contasOrdenadas() []string {
	nums := make([]string, 0, len(r.sessoes))
	for n := range r.sessoes {
		nums = append(nums, n)
	}
	sort.Strings(nums)
	return nums
}

func (r *repository) novoCliente(device *store.Device) *whatsmeow.Client {
	cli := whatsmeow.NewClient(device, waLog.Stdout("Client", r._logLevel, true))
	cli.AutomaticMessageRerequestFromPhone = true
	cli.AddEventHandler(func(rawEvt interface{}) { r.handler(cli, rawEvt) })
	return cli
}

func (r *repository) handler(cli *whatsmeow.Client, rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := cli.SendPresence(context.Background(), types.PresenceAvailable)
			if err != nil {
				log.Warnf("Failed to send available presence: %v", err)
			} else {
				log.Infof("Marked self as available")
			}
		}
	case *events.Connected, *events.PushNameSetting:
		if len(cli.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := cli.SendPresence(context.Background(), types.PresenceAvailable)
		if err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		} else {
			log.Infof("Marked self as available")

		}
	case *events.StreamReplaced:
		// A conta foi conectada em outro lugar: encerra SÓ esta sessão,
		// preservando as demais contas.
		if cli.Store.ID != nil {
			log.Warnf("stream substituída para a conta %s; desconectando a sessão", cli.Store.ID.User)
			r.Logout(context.Background(), cli.Store.ID.User)
		}
	case *events.Message:
		// Ponte do Copiloto: entrega a mensagem recebida ao PHP, que decide
		// se (e como) o copiloto responde. Nunca bloqueia o event loop.
		go r.encaminharPonte(cli, evt)

		metaParts := []string{fmt.Sprintf("pushname: %s", evt.Info.PushName), fmt.Sprintf("timestamp: %s", evt.Info.Timestamp)}
		if evt.Info.Type != "" {
			metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
		}
		if evt.Info.Category != "" {
			metaParts = append(metaParts, fmt.Sprintf("category: %s", evt.Info.Category))
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "view once")
		}
		if evt.IsViewOnce {
			metaParts = append(metaParts, "ephemeral")
		}
		if evt.IsViewOnceV2 {
			metaParts = append(metaParts, "ephemeral (v2)")
		}
		if evt.IsDocumentWithCaption {
			metaParts = append(metaParts, "document with caption")
		}
		if evt.IsEdit {
			metaParts = append(metaParts, "edit")
		}

		log.Infof("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)
	case *events.Receipt:
		if evt.Type == events.ReceiptTypeRead || evt.Type == events.ReceiptTypeReadSelf {
			log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
		} else if evt.Type == events.ReceiptTypeDelivered {
			log.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
		}
	case *events.Presence:
		if evt.Unavailable {
			if evt.LastSeen.IsZero() {
				log.Infof("%s is now offline", evt.From)
			} else {
				log.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
			}
		} else {
			log.Infof("%s is now online", evt.From)
		}
	case *events.AppState:
		log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	case *events.KeepAliveTimeout:
		log.Debugf("Keepalive timeout event: %+v", evt)
	case *events.KeepAliveRestored:
		log.Debugf("Keepalive restored")
	case *events.Blocklist:
		log.Infof("Blocklist event: %+v", evt)
	}
}
