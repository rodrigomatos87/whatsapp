package models

import "go.mau.fi/whatsmeow/types"

type Contact struct {
	Contact string
	types.ContactInfo
}

func (Contact) ListToResponse(list map[types.JID]types.ContactInfo) []Contact {
	contatos := []Contact{}
	for l, v := range list {

		contatos = append(contatos, Contact{
			Contact:     l.ToNonAD().String(),
			ContactInfo: v,
		})
	}

	return contatos
}
