package services

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"server/types"
	"time"
)

const (
	CONNECTION_TYPE = "tcp"
	HOST            = "localhost"
	PORT            = "8010"
)

const (
	CMD_AGENT_LOGIN  = "cmd_agent_login"
	CMD_AGENT_LOGOFF = "cmd_agent_logoff"
	CMD_AGENT_PAUSE  = "cmd_agent_pause"

	CMD_ACCEPT_CONVERSATION = "cmd_accept_conversation"
	CMD_REJECT_CONVERSATION = "cmd_reject_conversation"
	CMD_FINISH_CONVERSATION = "cmd_finish_conversation"

	CMD_GET_CUSTOMER_HISTORY = "cmd_get_customer_history"
	CMD_UPDATE_CONTACT_INFO  = "cmd_update_contact"
	CMD_SEND_MESSAGE         = "cmd_send_message"

	EVENT_CONVERSATION_STARTED  = "event_conversation_started"
	EVENT_CONVERSATION_ACCEPTED = "event_conversation_accepted"
	EVENT_CONVERSATION_FINISHED = "event_conversation_finished"
)

type Call struct {
	CallId       string
	CallerName   string
	CallerNumber string
	CalleeName   string
	CalleeNumber string
	Timestamp    uint
	Duration     int
	ContactID    string
}

var TcpServer TCPServer

type TCPServer struct {
	Listener net.Listener
}

func (server *TCPServer) Start() {

	// Listen for incoming connections
	var err error
	server.Listener, err = net.Listen(CONNECTION_TYPE, HOST+":"+PORT)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Close the Listener when the application closes
	// A defer statement defers the execution of a function until the surrounding function returns. (defer - odgadja)
	// The deferred call's arguments are evaluated immediately, but the function call is not executed until the surrounding function returns.
	// defer Listener.Close() this is moved outside this function , because i want to call this when the main function finishes (application)

	log.Println("Starting TCP Server on " + HOST + ":" + PORT)

	for {
		//Accept incoming connections
		conn, err := server.Listener.Accept()
		if err != nil {
			log.Fatal(err.Error())
		}

		// Handle connections in a new gouroutine
		// Goroutine is something similar to a thread
		// This is needed to ensure that server can accept more connections (clients)
		go server.handleClientRequest(conn)
	}
}

func (s *TCPServer) handleClientRequest(con net.Conn) {

	defer con.Close()

	for {
		data, err := bufio.NewReader(con).ReadString('\n')
		if err != nil {
			log.Println(err)
			return
		}

		log.Println("Received new data from an agent: ", data)

		var parsedData map[string]interface{}
		json.Unmarshal([]byte(data), &parsedData)

		action := parsedData["action"].(string)

		if action == CMD_AGENT_LOGIN {

			extension := parsedData["ext"].(string)
			secret := parsedData["secret"].(string)

			if agent := Asterisk.GetAgent(extension); agent != nil {
				if agent.password == secret {

					params := map[string]string{"Queue": "SalesQueue", "Interface": "PJSIP/" + extension}

					success := Asterisk.SendActionToManager("QueueAdd", params)

					parsedData["success"] = success

					if success {
						agent.socket = con

						parsedData["username"] = agent.Name
						parsedData["agents"] = Asterisk.Agents

						agent.Status = LoggedIn

						time.AfterFunc(5*time.Second, func() {
							if !agent.IsBusy() {
								Omnichannel.sendFirstWaitingConversation(agent.Ext)
							}
						})
					}

				} else {
					parsedData["success"] = false
				}
			} else {
				parsedData["success"] = false
			}

		} else if action == CMD_AGENT_LOGOFF {

			extension := parsedData["ext"].(string)

			params := map[string]string{"Queue": "SalesQueue", "Interface": "PJSIP/" + extension}
			success := Asterisk.SendActionToManager("QueueRemove", params)

			parsedData["success"] = success

			if success {
				if agent := Asterisk.GetAgent(extension); agent != nil {
					agent.Status = LoggedOut
					agent.OnCall = false
					agent.Paused = false
					agent.Conversations = 0
				}
			}
		} else if action == CMD_AGENT_PAUSE {

			extension := parsedData["ext"].(string)
			paused := parsedData["paused"].(bool)

			var pausedString string
			if paused {
				pausedString = "true"
			} else {
				pausedString = "false"
			}

			params := map[string]string{"Queue": "SalesQueue", "Interface": "PJSIP/" + extension, "Paused": pausedString}
			success := Asterisk.SendActionToManager("QueuePause", params)

			parsedData["success"] = success

			if success {

				if agent := Asterisk.GetAgent(extension); agent != nil {
					agent.Paused = paused

					time.AfterFunc(5*time.Second, func() {
						if !agent.IsBusy() {
							Omnichannel.sendFirstWaitingConversation(agent.Ext)
						}
					})
				}
			}
		} else if action == CMD_ACCEPT_CONVERSATION {

			conversationId := parsedData["conversationID"].(string)
			ext := parsedData["ext"].(string)

			if conversation := Omnichannel.AcceptConversation(conversationId, ext); conversation != nil {

				if agent := Asterisk.GetAgent(ext); agent != nil {
					agent.Conversations++
					log.Println("Agent conversations: ", agent.Conversations)
				}

				jsonData := make(map[string]interface{})
				jsonData["event"] = EVENT_CONVERSATION_ACCEPTED
				jsonData["conversationID"] = conversationId
				jsonData["ext"] = ext

				s.SendEventToAgents(jsonData, "", false)

				time.AfterFunc(5*time.Second, func() {
					Omnichannel.sendFirstWaitingConversation("")
				})
			}

		} else if action == CMD_REJECT_CONVERSATION {

			ext := parsedData["ext"].(string)

			if agent := Asterisk.GetAgent(ext); agent != nil {
				if !agent.IsBusy() {
					time.AfterFunc(5*time.Second, func() {
						Omnichannel.sendFirstWaitingConversation(ext)
					})
				}
			}

		} else if action == CMD_FINISH_CONVERSATION {

			conversationId := parsedData["conversationID"].(string)
			ext := parsedData["ext"].(string)

			if agent := Asterisk.GetAgent(ext); agent != nil {
				if agent.Conversations > 0 {
					agent.Conversations--
					log.Println("Agent conversations: ", agent.Conversations)
				}
			}

			Omnichannel.RemoveConversation(conversationId)

			jsonData := make(map[string]interface{})
			jsonData["event"] = EVENT_CONVERSATION_FINISHED
			jsonData["conversationID"] = conversationId
			s.SendEventToAgents(jsonData, ext, false)

			time.AfterFunc(5*time.Second, func() {
				Omnichannel.sendFirstWaitingConversation("")
			})

		} else if action == CMD_UPDATE_CONTACT_INFO {

			contactId := parsedData["contactID"].(string)
			var name, number string

			if parsedData["name"] != nil {
				name = parsedData["name"].(string)
				if name != "" {
					ContactDirectory.UpdateContact(contactId, "name", name)
				}
			}

			if parsedData["number"] != nil {
				number = parsedData["number"].(string)

				if existContact := ContactDirectory.FindContact("number", number); existContact.Id != "" {
					CallHistory.UpdateCallContactID(existContact.Id, contactId)
					ConversationHistory.UpdateConversationContactID(existContact.Id, contactId)
					ContactDirectory.DeleteContact(existContact.Id)
				}

				ContactDirectory.UpdateContact(contactId, "number", number)
			}
		} else if action == CMD_GET_CUSTOMER_HISTORY {

			contactId := parsedData["contactID"].(string)

			var conversations []*types.Conversation
			var calls []*Call

			log.Println("Get customer history ", contactId)

			conversations = ConversationHistory.GetConversationsByContactID(contactId)
			calls = CallHistory.GetCallsByContactID(contactId)

			parsedData["calls"] = calls
			parsedData["conversations"] = conversations

		} else if action == CMD_SEND_MESSAGE {

			conversationId := parsedData["conversationID"].(string)
			messageTxt := parsedData["text"].(string)

			Omnichannel.sendMessage(conversationId, messageTxt, false)
		}

		response, err := json.Marshal(parsedData)
		if err == nil {
			con.Write(response)
		}
	}
}

func (t *TCPServer) SendEventToAgents(jsonData map[string]interface{}, agentExt string, checkStatus bool) {

	data, err := json.Marshal(jsonData)
	if err == nil {

		if agentExt == "" {

			for i := range Asterisk.Agents {

				agent := Asterisk.Agents[i]

				if agent.Status == LoggedIn {

					if checkStatus {
						if !agent.IsBusy() {
							agent.socket.Write(data)
						}

					} else {
						agent.socket.Write(data)
					}
				}
			}

		} else {

			if agent := Asterisk.GetAgent(agentExt); agent != nil {
				agent.socket.Write(data)
			}
		}

	}
}
