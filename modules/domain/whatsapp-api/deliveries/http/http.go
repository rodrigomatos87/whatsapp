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

	// Rotas legadas: operam sempre na conta PRINCIPAL (conta = ""). É o que
	// mantém a frota inteira (alertas, API, telas antigas) funcionando sem
	// saber que o multi-conta existe.
	http.Echo().GET("/send", handler.send)
	http.Echo().POST("/send", handler.sendForm)
	http.Echo().GET("/check-status", handler.status)
	http.Echo().GET("/logout", handler.logout)
	http.Echo().GET("/queryInviteLink", handler.groupInfoByLink)
	http.Echo().GET("/info", handler.deviceInfo)
	http.Echo().GET("/getQRCode", handler.QrCode)
	http.Echo().GET("/contacts", handler.contacts)
	http.Echo().GET("/check-number/:phone", handler.checkNumber)

	// Multi-conta (modo avançado do Ravi). Tudo por path param: o proxy
	// nginx do /whatsapp não repassa query string.
	http.Echo().GET("/contas", handler.contas)
	http.Echo().GET("/contas/pareamento/qr", handler.qrContaNova)
	http.Echo().GET("/conta/:num/check-status", handler.status)
	http.Echo().GET("/conta/:num/info", handler.deviceInfo)
	http.Echo().GET("/conta/:num/contacts", handler.contacts)
	http.Echo().GET("/conta/:num/logout", handler.logout)
	http.Echo().GET("/conta/:num/principal", handler.definirPrincipal)
	http.Echo().GET("/conta/:num/send", handler.send)
	http.Echo().POST("/conta/:num/send", handler.sendForm)
	http.Echo().POST("/send-imagem", handler.enviarImagem)
	http.Echo().POST("/conta/:num/send-imagem", handler.enviarImagem)
	http.Echo().GET("/conta/:num/check-number/:phone", handler.checkNumber)
	http.Echo().GET("/conta/:num/queryInviteLink", handler.groupInfoByLink)
}

// conta extrai o número da conta da rota (vazio nas rotas legadas =
// conta principal).
func conta(c echo.Context) string {
	return c.Param("num")
}

func (h handler) contas(c echo.Context) error {
	ctx := c.Request().Context()
	lista, pareando, err := h.useCase.ListarContas(ctx)
	if err != nil {
		return h.InternalErr(c, err)
	}

	response := map[string]interface{}{
		"contas":   lista,
		"pareando": pareando,
	}

	return h.Response(c, "contas de WhatsApp", response)
}

func (h handler) definirPrincipal(c echo.Context) error {
	ctx := c.Request().Context()
	if err := h.useCase.DefinirPrincipal(ctx, conta(c)); err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, "conta principal definida")
}

func (h handler) qrContaNova(c echo.Context) error {
	ctx := c.Request().Context()
	qr, err := h.useCase.QrCode(ctx, true)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return c.String(http.StatusOK, qr)
}

func (h handler) checkNumber(c echo.Context) error {
	ctx := c.Request().Context()
	// Número vem no path (não na query): o proxy nginx do /whatsapp usa
	// proxy_pass com variável e não repassa a query string.
	phone := c.Param("phone")
	if phone != "" && phone[0] != '+' {
		phone = "+" + phone
	}

	res, err := h.useCase.CheckNumber(ctx, conta(c), phone)
	if err != nil {
		return h.InternalErr(c, err)
	}

	response := map[string]interface{}{
		"onWhatsApp": res.OnWhatsApp,
		"jid":        res.JID,
		"query":      res.Query,
	}

	return h.Response(c, "verificação de número", response)
}

// enviarImagem manda uma imagem LOCAL (pasta de mídia do copiloto) com legenda —
// é como os gráficos gerados no servidor chegam ao WhatsApp do usuário.
func (h handler) enviarImagem(c echo.Context) error {
	ctx := c.Request().Context()
	jid := c.Request().FormValue("jid")
	caminho := c.Request().FormValue("caminho")
	legenda := c.Request().FormValue("legenda")

	result, err := h.useCase.EnviarImagem(ctx, conta(c), jid, caminho, legenda)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, result)
}

func (h handler) sendForm(c echo.Context) error {
	ctx := c.Request().Context()
	jid := c.Request().FormValue("jid")
	text := c.Request().FormValue("text")

	result, err := h.useCase.Send(ctx, conta(c), jid, text)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, result)
}

func (h handler) send(c echo.Context) error {
	ctx := c.Request().Context()
	text := c.QueryParam("text")
	jid := c.QueryParam("jid")

	result, err := h.useCase.Send(ctx, conta(c), jid, text)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, result)
}

func (h handler) status(c echo.Context) error {
	ctx := c.Request().Context()
	status, err := h.useCase.Status(ctx, conta(c))
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
	JID, err := h.useCase.DeviceInfo(ctx, conta(c))
	if err != nil {
		return h.InternalErr(c, err)
	}

	response := map[string]interface{}{"ConnectedJID": JID.String()}

	return h.Response(c, "dispositivo conectado", response)
}

func (h handler) groupInfoByLink(c echo.Context) error {
	ctx := c.Request().Context()
	link := c.QueryParam("link")
	info, err := h.useCase.GroupInfoByLink(ctx, conta(c), link)
	if err != nil {
		return h.InternalErr(c, err)
	}

	result := map[string]interface{}{"groupInfo": info}

	return h.Response(c, "dados do grupo", result)
}

func (h handler) logout(c echo.Context) error {
	ctx := c.Request().Context()
	err := h.useCase.Logout(ctx, conta(c))
	if err != nil {
		return h.InternalErr(c, err)
	}

	return h.Response(c, "logout efetuado com sucesso")
}

func (h handler) QrCode(c echo.Context) error {
	ctx := c.Request().Context()
	qr, err := h.useCase.QrCode(ctx, false)
	if err != nil {
		return h.InternalErr(c, err)
	}

	return c.String(http.StatusOK, qr)
}

func (h handler) contacts(c echo.Context) error {
	ctx := c.Request().Context()

	groups, err := h.useCase.GetGroups(ctx, conta(c))
	if err != nil {
		return h.InternalErr(c, err)
	}

	groupsResp := models.Group{}.GroupListToResponse(groups)

	contacts, err := h.useCase.GetContacts(ctx, conta(c))
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
