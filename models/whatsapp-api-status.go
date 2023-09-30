package models

import "errors"

type WhatsAppStatusType string

const (
	WhatsAppStatusOK           WhatsAppStatusType = "OK"
	WhatsAppStatusNotConnected WhatsAppStatusType = "Não conectado"
	WhatsAppStatusOffline      WhatsAppStatusType = "Unauthorized"
)

type WhatsAppAPIStatus struct {
	Status    WhatsAppStatusType `json:"status"`
	Message   string             `json:"message"`
	Connected bool               `json:"connected"`
}

func (w WhatsAppAPIStatus) IsOK() error {
	if w.Status == WhatsAppStatusOK {
		return nil
	}

	return errors.New(w.Message)
}

func (WhatsAppAPIStatus) IsOnline() WhatsAppAPIStatus {
	return WhatsAppAPIStatus{
		Status:    WhatsAppStatusOK,
		Message:   "Cliente autenticado e conectado",
		Connected: true,
	}
}

func (WhatsAppAPIStatus) IsConnected() WhatsAppAPIStatus {
	return WhatsAppAPIStatus{
		Status:    WhatsAppStatusNotConnected,
		Message:   "Cliente conectado, mas não autenticado",
		Connected: true,
	}
}

func (WhatsAppAPIStatus) IsOffline() WhatsAppAPIStatus {
	return WhatsAppAPIStatus{
		Status:    WhatsAppStatusOffline,
		Message:   "Cliente desconectado",
		Connected: false,
	}
}
