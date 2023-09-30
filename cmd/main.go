package main

import (
	"flag"
	"fmt"
	"log"
	"ravi/modules/core/validators"
	"ravi/modules/domain/whatsapp-api/factory"
	httpserver "ravi/modules/server/http-server"
)

var logLevel = "INFO"
var debugLogs = flag.Bool("debug", false, "Enable debug logs?")
var dbDialect = flag.String("db-dialect", "sqlite3", "Database dialect (sqlite3 or postgres)")
var dbAddress = flag.String("db-address", "file:mdtest.db?_foreign_keys=on", "Database address")
var httpPort = flag.String("port", "9050", "Server address port")
var httpAddress = flag.String("address", "0.0.0.0", "Server address")

func main() {
	flag.Parse()

	if *debugLogs {
		logLevel = "DEBUG"
	}

	requestFullSync := false
	httpServer := httpserver.New(*debugLogs)

	validators := validators.New()
	if err := factory.New(httpServer, validators, logLevel, *dbDialect, *dbAddress, requestFullSync); err != nil {
		log.Fatal(err)
	}

	log.Fatal(httpServer.Start(fmt.Sprintf("%s:%s", *httpAddress, *httpPort)))
}
