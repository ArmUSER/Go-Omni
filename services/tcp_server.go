package services

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
)

const (
	CONNECTION_TYPE = "tcp"
	TCP_HOST        = "localhost"
	TCP_PORT        = "8010"
)

const (
	CMD_AGENT_LOGIN  = "cmd_agent_login"
	CMD_AGENT_LOGOFF = "cmd_agent_logoff"

	CMD_ACCEPT_CONVERSATION = "cmd_accept_conversation"
	CMD_FINISH_CONVERSATION = "cmd_finish_conversation"

	CMD_GET_MESSAGES         = "cmd_get_messages"
	CMD_GET_CUSTOMER_HISTORY = "cmd_get_customer_history"
	CMD_SEND_MESSAGE         = "cmd_send_message"

	EVENT_CONVERSATION_STARTED  = "event_conversation_started"
	EVENT_CONVERSATION_ACCEPTED = "event_conversation_accepted"
	EVENT_CONVERSATION_FINISHED = "event_conversation_finished"
)

type Agent struct {
	Id            string
	Name          string
	Conversations int
	socket        net.Conn
}

var TcpServer TCPServer

type TCPServer struct {
	Listener           net.Listener
	LoggedAgents       []*Agent
	loginAuthenticator LoginAuthenticator
}

func (server *TCPServer) Start() {

	server.InitializeLoginAuthenticator()

	var err error
	server.Listener, err = net.Listen(CONNECTION_TYPE, TCP_HOST+":"+TCP_PORT)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Starting TCP Server on " + TCP_HOST + ":" + TCP_PORT)

	for {
		conn, err := server.Listener.Accept()
		if err != nil {
			log.Fatal(err.Error())
		}

		go server.handleClientRequest(conn)
	}
}

func (server *TCPServer) Stop() {
	server.UnInitializeLoginAuthenticator()
	server.Listener.Close()
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

			username := parsedData["username"].(string)
			password := parsedData["password"].(string)

			agent, failedMsg := s.loginAuthenticator.Login(username, password)

			if agent != nil && failedMsg == "" {
				agent.socket = con
				s.LoggedAgents = append(s.LoggedAgents, agent)
				parsedData["agent"] = agent
				parsedData["conversations"] = Omnichannel.GetAgentActiveConversations(agent.Id)
				parsedData["success"] = 1
			} else {
				parsedData["login_failed_message"] = failedMsg
				parsedData["success"] = 0
			}

		} else if action == CMD_AGENT_LOGOFF {

			agentId := parsedData["id"].(string)
			success := s.loginAuthenticator.Logout(agentId)

			if success {
				for i := range s.LoggedAgents {
					if s.LoggedAgents[i].Id == agentId {
						s.LoggedAgents = append(s.LoggedAgents[:i], s.LoggedAgents[i+1:]...)
					}
				}
			}

			parsedData["success"] = success

		} else if action == CMD_ACCEPT_CONVERSATION {

			conversationId := parsedData["conversationID"].(string)
			agentId := parsedData["agentID"].(string)

			Omnichannel.AcceptConversation(conversationId, agentId)

			if agent := s.GetAgent(agentId); agent != nil {
				agent.Conversations++
			}

			jsonData := make(map[string]interface{})
			jsonData["event"] = EVENT_CONVERSATION_ACCEPTED
			jsonData["conversationID"] = conversationId
			jsonData["agentID"] = agentId

			s.SendEventToAgents(jsonData, "")

		} else if action == CMD_FINISH_CONVERSATION {

			conversationId := parsedData["conversationID"].(string)
			agentId := parsedData["agentID"].(string)

			if agent := s.GetAgent(agentId); agent != nil {
				if agent.Conversations > 0 {
					agent.Conversations--
				}
			}

			Omnichannel.FinishConversation(conversationId)

			jsonData := make(map[string]interface{})
			jsonData["event"] = EVENT_CONVERSATION_FINISHED
			jsonData["conversationID"] = conversationId
			s.SendEventToAgents(jsonData, agentId)

		} else if action == CMD_GET_MESSAGES {

			conversationId := parsedData["conversationID"].(string)
			parsedData["messages"] = Omnichannel.GetMessages(conversationId)

		} else if action == CMD_GET_CUSTOMER_HISTORY {

			customerID := parsedData["customer_id"].(string)
			parsedData["conversations"] = Omnichannel.GetCustomerConversations(customerID)

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

func (s *TCPServer) SendEventToAgents(jsonData map[string]interface{}, agentId string) {

	data, err := json.Marshal(jsonData)
	if err == nil {
		if agentId == "" {
			for i := range s.LoggedAgents {
				s.LoggedAgents[i].socket.Write(data)
			}
		} else {
			if agent := s.GetAgent(agentId); agent != nil {
				agent.socket.Write(data)
			}
		}
	}
}

func (s *TCPServer) GetAgent(id string) *Agent {
	for i := range s.LoggedAgents {
		if s.LoggedAgents[i].Id == id {
			return s.LoggedAgents[i]
		}
	}

	return nil
}

func (s *TCPServer) InitializeLoginAuthenticator() {
	s.loginAuthenticator = &AsteriskAuthenticator{}
	s.loginAuthenticator.Init()
}

func (s *TCPServer) UnInitializeLoginAuthenticator() {
	s.loginAuthenticator.Disconnect()
}
