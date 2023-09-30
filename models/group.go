package models

import "go.mau.fi/whatsmeow/types"

type Group struct {
	Contact string
	types.GroupInfo
}

func (Group) GroupListToResponse(list []*types.GroupInfo) []Group {
	contatos := []Group{}
	for _, v := range list {

		contatos = append(contatos, Group{
			Contact:   v.JID.ToNonAD().String(),
			GroupInfo: *v,
		})
	}

	return contatos
}
