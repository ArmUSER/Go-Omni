package services

import (
	"io/ioutil"
	"log"
	"net/http"
	"server/models"
	"server/types"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	HTTP_PORT = "8181"
	OMNI_DB   = "omni"

	EVENT_NEW_CONVERSATION       = "EVENT_NEW_CONVERSATION"
	EVENT_NEW_MESSAGE            = "EVENT_NEW_MESSAGE"
	EVENT_MESSAGE_STATUS_UPDATED = "EVENT_MESSAGE_STATUS_UPDATED"
)

const MESSAGE_CAPACITY = 2

var Omnichannel OmniChannel

type OmniChannel struct {
	Conversations    map[string]*types.Conversation //currently active conversations
	Queue            models.Queue
	Channels         map[types.ChannelType]Channel
	ConversationInfo map[string]string // conversationID => customerID
}

func startHttpServer() {

	log.Println("Starting HTTP Server at PORT " + HTTP_PORT)

	http.HandleFunc("/", HandleRequests)

	if err := http.ListenAndServe(":"+HTTP_PORT, nil); err != nil {
		log.Fatal("HTTP Server error:", err)
	}
}

func HandleRequests(w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	channelType := Omnichannel.getChannelType(req.URL.String())
	channel := Omnichannel.Channels[channelType]

	event, data := channel.parseReceivedData(body)

	senderUniqueID, senderName := channel.getSenderInfo(data)

	var contact types.Contact
	if contact = channel.findContact(senderUniqueID); contact.Id == "" {
		contact = channel.createContact(senderUniqueID, senderName)
		ContactDirectory.AddContact(contact)
	}

	customerID := Omnichannel.generateID(channelType, senderUniqueID)

	if event == EVENT_NEW_MESSAGE {

		messageText, timestamp := channel.getMessageInfo(data)

		var conversation *types.Conversation = nil

		if conversation = Omnichannel.FindConversation(customerID); conversation == nil {

			conversation = &types.Conversation{}

			conversation.Id = uuid.New().String()
			conversation.Type = channelType
			conversation.State = types.Default
			conversation.Customer = contact
			conversation.Created_Timestamp = timestamp

			Omnichannel.addNewMessage(conversation, Omnichannel.createEventMessage(EVENT_CONVERSATION_STARTED, timestamp))
		}

		message := types.Message{}
		message.Type = types.Text
		message.Text = messageText
		message.Timestamp = timestamp
		message.Status = types.Sent
		message.SentFromAgent = false

		if conversation != nil {

			Omnichannel.addNewMessage(conversation, message)

			if conversation.State == types.ConnectedToAgent || conversation.State == types.InQueue {

				/*
					jsonData["message"] = conversation.Messages[len(conversation.Messages)-1]
				*/

				jsonData := make(map[string]interface{})
				jsonData["event"] = event
				jsonData["conversationID"] = conversation.Id
				jsonData["message"] = message

				if conversation.State == types.ConnectedToAgent {
					TcpServer.SendEventToAgents(jsonData, conversation.ConnectedAgent, false)
				} else {
					TcpServer.SendEventToAgents(jsonData, "", false)
				}

			} else if conversation.State == types.Default {

				Omnichannel.Conversations[customerID] = conversation
				Omnichannel.ConversationInfo[conversation.Id] = customerID
				ConversationHistory.AddConversationToDB(conversation)

				conversation.State = types.InQueue
				ConversationHistory.UpdateConversationStateInDB(conversation.Id, conversation.State, "")

				positionInQueue := Omnichannel.Queue.Push(customerID)

				Omnichannel.sendMessage(conversation.Id, "Thank you for contacting us. One of agents will answer as soon as possible.", true)

				if positionInQueue == 1 {
					Omnichannel.sendFirstWaitingConversation("")
				}
			}
		}

	} else if event == EVENT_MESSAGE_STATUS_UPDATED {

		status := channel.getMessageStatus(data)

		if conversation := Omnichannel.FindConversation(customerID); conversation != nil {

			for i := range conversation.Messages {
				message := &conversation.Messages[i]
				if message.Status < status && message.SentFromAgent {
					message.Status = status
				}
			}

			jsonData := make(map[string]interface{})
			jsonData["event"] = event
			jsonData["conversationID"] = conversation.Id
			jsonData["status"] = status
			TcpServer.SendEventToAgents(jsonData, "", false)
		}
	}
}

func (o *OmniChannel) InitializeChannels() {
	viberHandler := Viber{}
	whatsappHandler := WhatsApp{}

	o.Channels = make(map[types.ChannelType]Channel)
	o.Channels[types.Viber] = viberHandler
	o.Channels[types.WhatsApp] = whatsappHandler

	for index, channel := range o.Channels {
		channel = o.Channels[index]
		channel.Init()
	}
}

func (o *OmniChannel) Init() {

	DBConnector.CreateDB(OMNI_DB)

	queryContactsTable := `CREATE TABLE IF NOT EXISTS contacts(id VARCHAR(256) primary key, number TEXT, name TEXT, viberID TEXT)`
	queryCallsTable := `CREATE TABLE IF NOT EXISTS calls(id VARCHAR(256) primary key, contactID TEXT, connectedAgent TEXT, timestamp BIGINT, duration INT)`
	queryConversationsTable := `CREATE TABLE IF NOT EXISTS conversations(id VARCHAR(256) primary key, type INT, contactID TEXT, connectedAgent TEXT, created_timestamp BIGINT, state INT)`
	queryMessagesTable := `CREATE TABLE IF NOT EXISTS messages(id INT primary key auto_increment, conversation_id VARCHAR(256), body TEXT, timestamp BIGINT, status INT, sentFromAgent BOOLEAN, type INT, event TEXT)`

	DBConnector.CreateTable(OMNI_DB, queryContactsTable)
	DBConnector.CreateTable(OMNI_DB, queryCallsTable)
	DBConnector.CreateTable(OMNI_DB, queryConversationsTable)
	DBConnector.CreateTable(OMNI_DB, queryMessagesTable)

	o.Conversations = make(map[string]*types.Conversation)
	o.ConversationInfo = make(map[string]string)
	o.InitializeChannels()
}

func (o *OmniChannel) Start() {
	o.Init()
	startHttpServer()
}

func (o *OmniChannel) generateID(channelType types.ChannelType, senderUniqueID string) string {
	sessionId := strconv.Itoa(int(channelType)) + "/" + senderUniqueID
	return uuid.NewSHA1(uuid.Nil, []byte(sessionId)).String()
}

func (o *OmniChannel) getChannelType(urlString string) types.ChannelType {
	var channelType types.ChannelType

	if strings.HasPrefix(urlString, "/viber") {
		channelType = types.Viber
	} else if strings.HasPrefix(urlString, "/whatsapp") {
		channelType = types.WhatsApp
	} else {
		channelType = types.Unknown
	}

	return channelType
}

func (o *OmniChannel) sendFirstWaitingConversation(toAgent string) {

	if !o.Queue.IsEmpty() {

		customerID := o.Queue.Elements[0].(string)

		if conversation := o.FindConversation(customerID); conversation != nil {

			jsonData := make(map[string]interface{})
			jsonData["event"] = EVENT_NEW_CONVERSATION
			jsonData["conversationID"] = conversation.Id
			jsonData["type"] = conversation.Type
			jsonData["customer"] = conversation.Customer
			jsonData["messages"] = conversation.Messages

			if toAgent == "" {
				TcpServer.SendEventToAgents(jsonData, "", true)
			} else {
				TcpServer.SendEventToAgents(jsonData, toAgent, false)
			}
		}
	}
}

func (o *OmniChannel) sendMessage(id string, messageTxt string, autoreply bool) {

	if conversation := o.FindConversation(o.ConversationInfo[id]); conversation != nil {

		channel := o.Channels[conversation.Type]
		channel.sendMessage(conversation.Customer, messageTxt, autoreply)

		if !autoreply {
			message := types.Message{}
			message.Type = types.Text
			message.Text = messageTxt
			message.Timestamp = uint(time.Now().UnixMilli())
			message.Status = types.Sent
			message.SentFromAgent = true

			o.addNewMessage(conversation, message)
		}
	}
}

func (o *OmniChannel) FindConversation(customerID string) *types.Conversation {

	if conversation, found := o.Conversations[customerID]; found {
		return conversation
	}

	return nil
}

func (o *OmniChannel) RemoveConversation(id string) {
	ConversationHistory.AddMessageToDB(id, o.createEventMessage(EVENT_CONVERSATION_FINISHED, uint(time.Now().UnixMilli())))
	ConversationHistory.UpdateConversationStateInDB(id, types.Finished, "")
	delete(o.Conversations, o.ConversationInfo[id])
	delete(o.ConversationInfo, id)
}

func (o *OmniChannel) AcceptConversation(id string, agentExt string) *types.Conversation {

	if conversation := o.FindConversation(o.ConversationInfo[id]); conversation != nil {
		conversation.State = types.ConnectedToAgent
		conversation.ConnectedAgent = agentExt
		ConversationHistory.UpdateConversationStateInDB(conversation.Id, conversation.State, conversation.ConnectedAgent)
		o.Queue.Pop()
		return conversation
	}

	return nil
}

func (o *OmniChannel) addNewMessage(conversation *types.Conversation, message types.Message) {
	conversation.Messages = append(conversation.Messages, message)
	ConversationHistory.AddMessageToDB(conversation.Id, message)
}

func (o *OmniChannel) createEventMessage(event string, timestamp uint) types.Message {

	message := types.Message{}
	message.Type = types.Event
	message.Timestamp = timestamp
	message.Status = types.Sent
	message.SentFromAgent = false
	message.Event = event
	return message
}
