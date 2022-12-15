package services

type LoginAuthenticator interface {
	Init()
	Login(username string, password string) (agent *Agent, failedMsg string)
	Logout(userId string) (success bool)
	Disconnect()
}
