package services

import (
	"log"
	"time"

	"github.com/bit4bit/gami"
	"github.com/go-ini/ini"
)

var (
	AST_SERVER_IP      string
	AST_PORT           string
	AMI_USER           string
	AMI_PASSWORD       string
	AST_DB_CREDENTIALS string
	AST_DB_NAME        string
)

type AsteriskAuthenticator struct {
	AMIClient *gami.AMIClient
}

func (a *AsteriskAuthenticator) Init() {
	cfg, err := ini.Load("login_auth_conf.ini")
	if err != nil {
		log.Println("Failed to read login_auth_conf file: ", err)
		return
	}

	AST_SERVER_IP = cfg.Section("asterisk-authenticator").Key("AST_SERVER_IP").String()
	AST_PORT = cfg.Section("asterisk-authenticator").Key("AST_PORT").String()
	AMI_USER = cfg.Section("asterisk-authenticator").Key("AMI_USER").String()
	AMI_PASSWORD = cfg.Section("asterisk-authenticator").Key("AMI_PASSWORD").String()

	cfg, err = ini.Load("db_config.ini")
	if err != nil {
		log.Println("Failed to read db_config file: ", err)
		return
	}

	dbUser := cfg.Section("asterisk-db").Key("DB_USER").String()
	dbPass := cfg.Section("asterisk-db").Key("DB_PASSWORD").String()
	dbConnType := cfg.Section("asterisk-db").Key("DB_CONNECTION_TYPE").String()
	dbServerIP := cfg.Section("asterisk-db").Key("DB_SERVER_IP").String()
	dbPort := cfg.Section("asterisk-db").Key("DB_PORT").String()

	AST_DB_CREDENTIALS = dbUser + ":" + dbPass + "@" + dbConnType + "(" + dbServerIP + ":" + dbPort + ")"
	AST_DB_NAME = cfg.Section("asterisk-db").Key("DB_NAME").String()

	a.ConnectToManager()
}

func (a *AsteriskAuthenticator) Login(username string, password string) (*Agent, string) {

	agent := Agent{}
	var failedMsg string

	results := DBConnector.ExecuteQuery(AST_DB_CREDENTIALS, AST_DB_NAME, "SELECT u.ext, u.name FROM users u INNER JOIN ps_auths p ON u.ext = p.id WHERE p.id='"+username+"' AND p.password='"+password+"'")
	defer results.Close()

	if results.Next() {
		var result QueryResult
		if err := results.Scan(&result.param1, &result.param2); err != nil {
			failedMsg = "Login failed: " + err.Error()
			return nil, failedMsg
		}

		params := map[string]string{"Queue": "SalesQueue", "Interface": "PJSIP/" + username}
		success := a.SendActionToManager("QueueAdd", params)

		if success {
			agent.Id = result.param1.String
			agent.Name = result.param2.String
			agent.Conversations = 0
		} else {
			failedMsg = "Login failed"
		}
	}

	return &agent, failedMsg
}

func (a *AsteriskAuthenticator) Logout(id string) bool {

	params := map[string]string{"Queue": "SalesQueue", "Interface": "PJSIP/" + id}
	success := a.SendActionToManager("QueueRemove", params)
	return success
}

func (a *AsteriskAuthenticator) Disconnect() {
	a.AMIClient.Close()
}

func (a *AsteriskAuthenticator) ConnectToManager() {

	var err error
	a.AMIClient, err = gami.Dial(AST_SERVER_IP + ":" + AST_PORT)
	if err != nil {
		log.Fatal("AMI Connection failed", err.Error())
	}

	a.AMIClient.Run()

	go func() {
		for {
			select {
			case err := <-a.AMIClient.NetError:
				log.Println("AMI Network Error:", err)
				<-time.After(time.Second)
				if err := a.AMIClient.Reconnect(); err == nil {
					a.AMIClient.Action("Events", gami.Params{"EventMask": "on"})
				}

			case err := <-a.AMIClient.Error:
				log.Println("AMI Error:", err)

			case event := <-a.AMIClient.Events:
				log.Println("AMI Event received: ", event)
				//a.handleAMIEvent(event) --> handle event received from Asterisk if needed
			}
		}
	}()

	if err := a.AMIClient.Login(AMI_USER, AMI_PASSWORD); err != nil {
		log.Fatal("AMI Login failed", err.Error())
	}
}

func (a *AsteriskAuthenticator) SendActionToManager(action string, params map[string]string) bool {

	var success bool

	if ami_response, err := a.AMIClient.Action(action, params); err != nil {
		log.Println("AMI Action error: ", ami_response.Status)
		success = false
	} else {
		log.Println("AMI Action success", ami_response)
		success = true
	}

	return success
}
