package repository

import (
	"context"
	"fmt"
	whatsappapi "ravi/modules/domain/whatsapp-api"
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

type repository struct {
	_logLevel      string
	_storage       *sqlstore.Container
	conn           *client
	qrChan         chan string
	pairRejectChan chan bool
	qrChanClose    chan bool
}

func New(logLevel, dbDialect, dbAddress string, requestFullSync bool) (whatsappapi.Repository, error) {
	waBinary.IndentXML = true
	store.DeviceProps.RequireFullSync = proto.Bool(false)
	store.DeviceProps.PlatformType = meowWaProto.DeviceProps_FIREFOX.Enum()
	store.SetOSInfo("Ravi Monitor", store.GetWAVersion())
	log = waLog.Stdout("Handler", logLevel, true)

	if requestFullSync {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
	}
	dbLog := waLog.Stdout("Database", logLevel, true)
	storeContainer, err := sqlstore.New(dbDialect, dbAddress, dbLog)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	repo := &repository{
		_logLevel:      logLevel,
		_storage:       storeContainer,
		conn:           &client{},
		qrChan:         make(chan string, 1),
		pairRejectChan: make(chan bool, 1),
		qrChanClose:    make(chan bool, 1),
	}

	conn, err := repo._connToWhatsApp()
	if err != nil {
		return nil, err
	}

	repo.conn.SetClient(conn)

	return repo, conn.Connect()
}

func (r *repository) handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		conn, err := r.conn.Client()
		if err != nil {
			log.Warnf("Failed to send available presence: %v", err)
			return
		}

		if len(conn.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := conn.SendPresence(types.PresenceAvailable)
			if err != nil {
				log.Warnf("Failed to send available presence: %v", err)
			} else {
				log.Infof("Marked self as available")
			}
		}
	case *events.Connected, *events.PushNameSetting:
		conn, err := r.conn.Client()
		if err != nil {
			log.Warnf("Failed to send available presence: %v", err)
			return
		}
		if len(conn.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err = conn.SendPresence(types.PresenceAvailable)
		if err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		} else {
			log.Infof("Marked self as available")

		}
	case *events.StreamReplaced:
		r.Logout(context.Background())
	case *events.Message:
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
