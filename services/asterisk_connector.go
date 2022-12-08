package services

import (
	"log"
	"net"
	"server/types"
	"strconv"
	"strings"
	"time"

	"github.com/bit4bit/gami"
	"github.com/google/uuid"
)

const (
	SERVER_IP     = "172.16.47.131"
	ASTERISK_PORT = "5038"
	AST_DB_NAME   = "asterisk"
	AMI_USER      = "admin"
	AMI_PASSWORD  = "test123"
)

const (
	EVENT_AGENT_CONNECT  = "AgentConnect"
	EVENT_AGENT_COMPLETE = "AgentComplete"
	EVENT_AGENT_CALLED   = "AgentCalled"
)

type AgentLoginStatus int

const (
	LoggedIn AgentLoginStatus = iota
	LoggedOut
)

type Agent struct {
	Ext           string
	Name          string
	Status        AgentLoginStatus
	OnCall        bool
	Paused        bool
	Conversations int //number of currently accepted conversations

	password string
	socket   net.Conn
}

func (a *Agent) IsBusy() bool {

	var busy bool = false

	if a.OnCall || a.Paused || (a.Conversations >= MESSAGE_CAPACITY) {
		busy = true
	}

	return busy
}

var Asterisk ASTERISK

type ASTERISK struct {
	AMIClient *gami.AMIClient
	Agents    []Agent
}

func (a *ASTERISK) ConnectToManager() {

	var err error
	a.AMIClient, err = gami.Dial(SERVER_IP + ":" + ASTERISK_PORT)
	if err != nil {
		log.Fatal("AMI Connection failed", err.Error())
	}

	a.AMIClient.Run()

	//install manager
	go func() {
		for {
			select {

			//Handle network errors
			case err := <-a.AMIClient.NetError:
				log.Println("AMI Network Error:", err)
				//Try a new connection every second
				<-time.After(time.Second)
				if err := a.AMIClient.Reconnect(); err == nil {
					//Call start actions
					a.AMIClient.Action("Events", gami.Params{"EventMask": "on"})
				}

			//Handle other errors
			case err := <-a.AMIClient.Error:
				log.Println("AMI Error:", err)

			//Wait for events and process
			case event := <-a.AMIClient.Events:
				a.handleAMIEvent(event)
			}
		}
	}()

	if err := a.AMIClient.Login(AMI_USER, AMI_PASSWORD); err != nil {
		log.Fatal("AMI Login failed", err.Error())
	}
}

func (a *ASTERISK) handleAMIEvent(event *gami.AMIEvent) {

	//log.Println("Received AMI event: ", event.ID, event.Params)

	if event.ID == EVENT_AGENT_CALLED {

		callerNumber := event.Params["Calleridnum"]
		agent := strings.ReplaceAll(event.Params["Interface"], "PJSIP/", "")
		callUid := event.Params["Uniqueid"]

		jsonData := make(map[string]interface{})
		jsonData["event"] = EVENT_NEW_CONVERSATION
		jsonData["conversationID"] = Omnichannel.generateID(types.PhoneCall, callerNumber)
		jsonData["type"] = types.PhoneCall

		var contact types.Contact
		if contact = ContactDirectory.FindContact("number", callerNumber); contact.Id == "" {
			contact = types.Contact{}
			contact.Id = uuid.New().String()
			contact.Number = callerNumber
			ContactDirectory.AddContact(contact)
		}

		jsonData["customer"] = contact
		jsonData["callId"] = callUid
		jsonData["callType"] = 1 //incoming

		TcpServer.SendEventToAgents(jsonData, agent, false)

	} else if event.ID == EVENT_AGENT_CONNECT {

		ext := strings.ReplaceAll(event.Params["Interface"], "PJSIP/", "")
		callerNumber := event.Params["Calleridnum"]

		if agent := a.GetAgent(ext); agent != nil {
			agent.OnCall = true
		}

		jsonData := make(map[string]interface{})
		jsonData["event"] = EVENT_CONVERSATION_ACCEPTED
		jsonData["conversationID"] = Omnichannel.generateID(types.PhoneCall, callerNumber)
		jsonData["ext"] = ext
		TcpServer.SendEventToAgents(jsonData, ext, false)

	} else if event.ID == EVENT_AGENT_COMPLETE {

		ext := strings.ReplaceAll(event.Params["Interface"], "PJSIP/", "")

		if agent := a.GetAgent(ext); agent != nil {
			agent.OnCall = false

			time.AfterFunc(5*time.Second, func() {
				if !agent.IsBusy() {
					Omnichannel.sendFirstWaitingConversation(agent.Ext)
				}
			})
		}

		var call Call

		callId := event.Params["Uniqueid"]
		callerNumber := event.Params["Calleridnum"]
		connectedAgent := ext
		timestamp := event.Params["Timestamp"]
		duration := event.Params["Talktime"]

		call.CallId = callId
		call.CalleeNumber = connectedAgent

		if time, err := strconv.Atoi(timestamp); err == nil {
			call.Timestamp = uint(time)
		}

		if durationTime, err := strconv.Atoi(duration); err == nil {
			call.Duration = durationTime
		}

		if contact := ContactDirectory.FindContact("number", callerNumber); contact.Id != "" {
			call.ContactID = contact.Id
		}

		CallHistory.AddCallToDB(&call)
	}
}

func (a *ASTERISK) SendActionToManager(action string, params map[string]string) bool {

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

func (a *ASTERISK) LoadConfigurationFromDB() {

	{
		// Load all agents
		results := DBConnector.ExecuteQuery(AST_DB_NAME, "SELECT u.ext, u.name , p.password FROM users u INNER JOIN ps_auths p ON u.ext = p.id")
		for results.Next() {
			var result QueryResult
			if err := results.Scan(&result.param1, &result.param2, &result.param3); err != nil {
				log.Println("Query result error", err)
				return
			}

			var agent Agent
			agent.Ext = result.param1.String
			agent.Name = result.param2.String
			agent.password = result.param3.String
			agent.Status = LoggedOut
			agent.OnCall = false
			agent.Paused = false
			agent.Conversations = 0

			a.Agents = append(a.Agents, agent)
		}
	}

}

func (a *ASTERISK) GetAgent(ext string) *Agent {

	var agent *Agent = nil

	for i := range a.Agents {
		if a.Agents[i].Ext == ext {
			agent = &a.Agents[i]
			break
		}
	}

	return agent
}
