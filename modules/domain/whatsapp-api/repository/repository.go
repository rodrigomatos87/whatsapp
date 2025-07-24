package repository

import (
	"context"
	"fmt"
	"os"
	"ravi/models"
	"strings"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func (r *repository) GetGroups(ctx context.Context) ([]*types.GroupInfo, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}

	if err := status.IsOK(); err != nil {
		return nil, err
	}

	conn, err := r.conn.Client()
	if err != nil {
		return nil, err
	}

	return conn.GetJoinedGroups()
}

func (r *repository) GetContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}

	if err := status.IsOK(); err != nil {
		return nil, err
	}

	conn, err := r.conn.Client()
	if err != nil {
		return nil, err
	}

	return conn.Store.Contacts.GetAllContacts(context.Background())
}

func (r *repository) Logout(ctx context.Context) (err error) {
	conn, err := r.conn.Client()
	if err != nil {
		return err
	}

	conn.Logout(context.Background())
	conn.Disconnect()
	conn.Store.Delete(context.Background())

	r.conn.SetClient(nil)
	r.conn.SetQrCode("")

	os.Exit(1)
	return err
}

func (r *repository) GroupInfoByLink(ctx context.Context, link string) (*types.GroupInfo, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}

	if err := status.IsOK(); err != nil {
		return nil, err
	}

	conn, err := r.conn.Client()
	if err != nil {
		return nil, err
	}

	resp, err := conn.GetGroupInfoFromLink(link)
	if err != nil {
		return nil, fmt.Errorf("erro ao resolver o link do convite do grupo: %v", err)
	}

	return resp, nil
}

func (r *repository) DeviceInfo(ctx context.Context) (*types.JID, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}

	if err := status.IsOK(); err != nil {
		return nil, err
	}

	conn, err := r.conn.Client()
	if err != nil {
		return nil, err
	}

	return conn.Store.ID, nil
}

func (r *repository) Status(ctx context.Context) (models.WhatsAppAPIStatus, error) {
	conn, err := r.conn.Client()
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

func (r *repository) Send(ctx context.Context, jid1, text string) (string, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return "", err
	}

	if err := status.IsOK(); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	jid, err := parseJID(jid1)
	if err != nil {
		return "", err
	}

	conn, err := r.conn.Client()
	if err != nil {
		return "", err
	}

	message := &waProto.Message{Conversation: proto.String(text)}

	if _, err := conn.SendMessage(ctx, jid, message); err != nil {
		return "", err
	}

	return "mensagem enviada com sucesso!", nil
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
