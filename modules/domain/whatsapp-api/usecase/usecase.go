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

func (u *useCase) Send(ctx context.Context, conta, jid, text string) (string, error) {
	if err := u.validators.String(jid, "jid"); err != nil {
		return "", err
	}

	if err := u.validators.String(text, "text"); err != nil {
		return "", err
	}

	return u.repo.Send(ctx, conta, jid, text)
}

func (u *useCase) Status(ctx context.Context, conta string) (models.WhatsAppAPIStatus, error) {
	return u.repo.Status(ctx, conta)
}

func (u *useCase) DeviceInfo(ctx context.Context, conta string) (*types.JID, error) {
	return u.repo.DeviceInfo(ctx, conta)
}

func (u *useCase) GroupInfoByLink(ctx context.Context, conta, link string) (*types.GroupInfo, error) {
	if err := u.validators.String(link, "link"); err != nil {
		return nil, err
	}

	return u.repo.GroupInfoByLink(ctx, conta, link)
}

func (u *useCase) Logout(ctx context.Context, conta string) error {
	return u.repo.Logout(ctx, conta)
}

func (u *useCase) QrCode(ctx context.Context, contaNova bool) (string, error) {
	return u.repo.RequestNewQRCode(ctx, contaNova)
}

func (u *useCase) GetContacts(ctx context.Context, conta string) (map[types.JID]types.ContactInfo, error) {
	return u.repo.GetContacts(ctx, conta)
}

func (u *useCase) GetGroups(ctx context.Context, conta string) ([]*types.GroupInfo, error) {
	return u.repo.GetGroups(ctx, conta)
}

func (u *useCase) CheckNumber(ctx context.Context, conta, phone string) (models.NumberCheck, error) {
	if err := u.validators.String(phone, "phone"); err != nil {
		return models.NumberCheck{}, err
	}

	return u.repo.CheckNumber(ctx, conta, phone)
}

func (u *useCase) ListarContas(ctx context.Context) ([]models.Conta, bool, error) {
	return u.repo.ListarContas(ctx)
}

func (u *useCase) DefinirPrincipal(ctx context.Context, conta string) error {
	if err := u.validators.String(conta, "conta"); err != nil {
		return err
	}

	return u.repo.DefinirPrincipal(ctx, conta)
}

func (u *useCase) EnviarImagem(ctx context.Context, conta, jid, caminho, legenda string) (string, error) {
	if err := u.validators.String(jid, "jid"); err != nil {
		return "", err
	}

	if err := u.validators.String(caminho, "caminho"); err != nil {
		return "", err
	}

	return u.repo.EnviarImagem(ctx, conta, jid, caminho, legenda)
}
