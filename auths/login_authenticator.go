package auths

import "server/types"

type LoginAuthenticator interface {
	Init()
	Login(username string, password string) (agent *types.Agent, failedMsg string)
	Logout(userId string) (success bool)
	Disconnect()
}
