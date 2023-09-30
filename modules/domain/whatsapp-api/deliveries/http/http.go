package httphandler

import (
	"net/http"
	"ravi/models"
	whatsappapi "ravi/modules/domain/whatsapp-api"
	httpserver "ravi/modules/server/http-server"

	"github.com/labstack/echo/v4"
)

type handler struct {
	httpserver.Server
	useCase whatsappapi.UseCase
}

func New(http httpserver.Server, u whatsappapi.UseCase) {
	handler := &handler{
		Server:  http,
		useCase: u,
	}

	http.Echo().GET("/send", handler.send)
	http.Echo().POST("/send", handler.sendForm)
	http.Echo().GET("/check-status", handler.status)
	http.Echo().GET("/logout", handler.logout)
	http.Echo().GET("/queryInviteLink", handler.groupInfoByLink)
	http.Echo().GET("/info", handler.deviceInfo)
	http.Echo().GET("/getQRCode", handler.QrCode)
	http.Echo().GET("/contacts", handler.contacts)
}

func (h handler) sendForm(c echo.Context) error {
	ctx := c.Request().Context()
	jid := c.Request().FormValue("jid")
	text := c.Request().FormValue("text")

	result, err := h.useCase.Send(ctx, jid, text)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, result)
}

func (h handler) send(c echo.Context) error {
	ctx := c.Request().Context()
	text := c.QueryParam("text")
	jid := c.QueryParam("jid")

	result, err := h.useCase.Send(ctx, jid, text)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, result)
}

func (h handler) status(c echo.Context) error {
	ctx := c.Request().Context()
	status, err := h.useCase.Status(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	response := map[string]interface{}{
		"status":    status.Status,
		"connected": status.Connected,
	}

	return h.Response(c, status.Message, response)

}

func (h handler) deviceInfo(c echo.Context) error {
	ctx := c.Request().Context()
	JID, err := h.useCase.DeviceInfo(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	response := map[string]interface{}{"ConnectedJID": JID.String()}

	return h.Response(c, "dispositivo conectado", response)
}

func (h handler) groupInfoByLink(c echo.Context) error {
	ctx := c.Request().Context()
	link := c.QueryParam("link")
	info, err := h.useCase.GroupInfoByLink(ctx, link)
	if err != nil {
		return h.InternalErr(c, err)
	}

	result := map[string]interface{}{"groupInfo": info}

	return h.Response(c, "dados do grupo", result)
}

func (h handler) logout(c echo.Context) error {
	ctx := c.Request().Context()
	err := h.useCase.Logout(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, "logout efetuado com sucesso")
}

func (h handler) QrCode(c echo.Context) error {
	ctx := c.Request().Context()
	qr, err := h.useCase.QrCode(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return c.String(http.StatusOK, qr)
}

func (h handler) contacts(c echo.Context) error {
	ctx := c.Request().Context()

	groups, err := h.useCase.GetGroups(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	groupsResp := models.Group{}.GroupListToResponse(groups)

	contacts, err := h.useCase.GetContacts(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	contactsResp := models.Contact{}.ListToResponse(contacts)

	allContacts := []interface{}{}

	for _, group := range groupsResp {
		allContacts = append(allContacts, group)
	}

	for _, contact := range contactsResp {
		allContacts = append(allContacts, contact)
	}

	response := map[string]interface{}{
		"contatos": allContacts,
	}

	return h.Response(c, "lista de contatos", response)
}
