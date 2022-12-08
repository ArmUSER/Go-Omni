package main

import (
	"server/services"

	_ "github.com/go-sql-driver/mysql"
)

func main() {

	services.Asterisk.ConnectToManager()
	defer services.Asterisk.AMIClient.Close()

	services.Asterisk.LoadConfigurationFromDB()

	go services.Omnichannel.Start()

	services.TcpServer.Start()
	defer services.TcpServer.Listener.Close()
}
