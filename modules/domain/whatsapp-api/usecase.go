package whatsappapi

import (
	"context"
	"ravi/models"

	"go.mau.fi/whatsmeow/types"
)

type UseCase interface {
	Send(ctx context.Context, jid, text string) (string, error)
	Status(ctx context.Context) (models.WhatsAppAPIStatus, error)
	DeviceInfo(ctx context.Context) (*types.JID, error)
	GroupInfoByLink(ctx context.Context, link string) (*types.GroupInfo, error)
	Logout(ctx context.Context) error
	QrCode(context.Context) (string, error)
	GetContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error)
	GetGroups(ctx context.Context) ([]*types.GroupInfo, error)
}
