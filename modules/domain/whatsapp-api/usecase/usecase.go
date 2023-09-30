package usecase

import (
	"context"
	"ravi/models"
	whatsappapi "ravi/modules/domain/whatsapp-api"

	"go.mau.fi/whatsmeow/types"
)

type useCase struct {
	validators models.Validators
	repo       whatsappapi.Repository
}

func New(v models.Validators, r whatsappapi.Repository) whatsappapi.UseCase {
	return &useCase{
		validators: v,
		repo:       r,
	}
}

func (u *useCase) Send(ctx context.Context, jid, text string) (string, error) {
	if err := u.validators.String(jid, "jid"); err != nil {
		return "", err
	}

	if err := u.validators.String(text, "text"); err != nil {
		return "", err
	}

	return u.repo.Send(ctx, jid, text)
}

func (u *useCase) Status(ctx context.Context) (models.WhatsAppAPIStatus, error) {
	return u.repo.Status(ctx)
}

func (u *useCase) DeviceInfo(ctx context.Context) (*types.JID, error) {
	return u.repo.DeviceInfo(ctx)
}

func (u *useCase) GroupInfoByLink(ctx context.Context, link string) (*types.GroupInfo, error) {
	if err := u.validators.String(link, "link"); err != nil {
		return nil, err
	}

	return u.repo.GroupInfoByLink(ctx, link)
}

func (u *useCase) Logout(ctx context.Context) error {
	return u.repo.Logout(ctx)
}

func (u *useCase) QrCode(ctx context.Context) (string, error) {
	return u.repo.RequestNewQRCode(ctx)
}

func (u *useCase) GetContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error) {
	return u.repo.GetContacts(ctx)
}

func (u *useCase) GetGroups(ctx context.Context) ([]*types.GroupInfo, error) {
	return u.repo.GetGroups(ctx)
}
