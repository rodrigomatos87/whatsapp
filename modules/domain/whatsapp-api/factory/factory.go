package factory

import (
	"ravi/models"
	httphandler "ravi/modules/domain/whatsapp-api/deliveries/http"
	"ravi/modules/domain/whatsapp-api/repository"
	"ravi/modules/domain/whatsapp-api/usecase"
	httpserver "ravi/modules/server/http-server"
)

func New(http httpserver.Server, v models.Validators,
	logLevel,
	dbDialect,
	dbAddress string,
	requestFullSync bool,
	ponteURL string,
) error {
	repo, err := repository.New(logLevel, dbDialect, dbAddress, requestFullSync, ponteURL)
	if err != nil {
		return err
	}

	usecase := usecase.New(v, repo)

	httphandler.New(http, usecase)

	return nil
}
