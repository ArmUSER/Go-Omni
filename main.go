package main

import (
	"server/services"

	_ "github.com/go-sql-driver/mysql"
)

func main() {

	go services.Omnichannel.Start()

	services.TcpServer.Start()
	defer services.TcpServer.Stop()
}
