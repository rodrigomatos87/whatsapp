package whatsappapi

import (
	"context"
	"ravi/models"

	"go.mau.fi/whatsmeow/types"
)

// conta = número da conta de WhatsApp (JID.User); vazio = conta principal.
type Repository interface {
	Send(ctx context.Context, conta, jid, text string) (string, error)
	Status(ctx context.Context, conta string) (models.WhatsAppAPIStatus, error)
	DeviceInfo(ctx context.Context, conta string) (*types.JID, error)
	GroupInfoByLink(ctx context.Context, conta, link string) (*types.GroupInfo, error)
	Logout(ctx context.Context, conta string) error
	GetContacts(ctx context.Context, conta string) (map[types.JID]types.ContactInfo, error)
	GetGroups(ctx context.Context, conta string) ([]*types.GroupInfo, error)
	RequestNewQRCode(ctx context.Context, contaNova bool) (string, error)
	CheckNumber(ctx context.Context, conta, phone string) (models.NumberCheck, error)
	ListarContas(ctx context.Context) ([]models.Conta, bool, error)
	DefinirPrincipal(ctx context.Context, conta string) error
	EnviarImagem(ctx context.Context, conta, jid, caminho, legenda string) (string, error)
}
