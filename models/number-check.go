package models

// NumberCheck é o resultado da verificação de um número no WhatsApp
// (via IsOnWhatsApp). JID traz o ID canônico (ex.: 5544...@s.whatsapp.net),
// já normalizado pelo WhatsApp (resolve o 9º dígito do Brasil).
type NumberCheck struct {
	Query      string `json:"query"`
	JID        string `json:"jid"`
	OnWhatsApp bool   `json:"onWhatsApp"`
}
