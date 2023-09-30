package httpserver

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server interface {
	Response(c echo.Context, message string, data ...map[string]interface{}) error
	InternalErr(c echo.Context, err error) error
	Echo() *echo.Echo
	Start(address string) error
}

type server struct {
	e *echo.Echo
}

func New(debug bool) Server {
	e := echo.New()
	e.HideBanner = true

	if debug {
		e.Use(middleware.Logger())
		e.Use(middleware.Recover())
	}

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		Skipper:      middleware.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
	}))
	return &server{
		e: e,
	}
}

func (s server) Echo() *echo.Echo {
	return s.e
}

func (s server) Start(address string) error {
	return s.e.Start(address)
}

func (s server) Response(c echo.Context, message string, data ...map[string]interface{}) error {
	return s.response(c, http.StatusOK, s.buildResponse(true, message, data...))
}

func (s server) InternalErr(c echo.Context, err error) error {
	return s.response(c, http.StatusInternalServerError, s.buildResponse(false, err.Error()))
}

func (s server) response(c echo.Context, code int, resp map[string]interface{}) error {
	return c.JSON(code, resp)
}

func (s server) buildResponse(success bool, message string, data ...map[string]interface{}) map[string]interface{} {
	response := make(map[string]interface{})
	response["success"] = success
	response["message"] = message

	if len(data) > 0 && data[0] != nil {
		for label, value := range data[0] {
			response[label] = value
		}
	}

	return response
}
