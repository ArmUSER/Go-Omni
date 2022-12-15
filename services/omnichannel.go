package services

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"server/types"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/google/uuid"
)

const (
	HTTP_PORT = "8181"

	EVENT_NEW_CONVERSATION       = "EVENT_NEW_CONVERSATION"
	EVENT_NEW_MESSAGE            = "EVENT_NEW_MESSAGE"
	EVENT_MESSAGE_STATUS_UPDATED = "EVENT_MESSAGE_STATUS_UPDATED"
)

var (
	OMNI_DB_CREDENTIALS string
	OMNI_DB_NAME        string
)

var executionTime float64

var Omnichannel OmniChannel

type OmniChannel struct {
	Channels map[types.ChannelType]Channel //map[channelType]channelHandler
}

func (o *OmniChannel) Start() {
	o.Init()
	startHttpServer()
}

func (o *OmniChannel) Init() {

	cfg, err := ini.Load("db_config.ini")
	if err != nil {
		log.Println("Failed to read db_config file: ", err)
		return
	}

	dbUser := cfg.Section("omni-db").Key("DB_USER").String()
	dbPass := cfg.Section("omni-db").Key("DB_PASSWORD").String()
	dbConnType := cfg.Section("omni-db").Key("DB_CONNECTION_TYPE").String()
	dbServerIP := cfg.Section("omni-db").Key("DB_SERVER_IP").String()
	dbPort := cfg.Section("omni-db").Key("DB_PORT").String()

	OMNI_DB_CREDENTIALS = dbUser + ":" + dbPass + "@" + dbConnType + "(" + dbServerIP + ":" + dbPort + ")"
	OMNI_DB_NAME = cfg.Section("omni-db").Key("DB_NAME").String()

	DBConnector.CreateDB(OMNI_DB_CREDENTIALS, OMNI_DB_NAME)

	queryCustomersTable := `CREATE TABLE IF NOT EXISTS customer(id VARCHAR(256) primary key, name TEXT, INDEX index_c1 (name(255)))`
	queryCustomerContactsTable := `CREATE TABLE IF NOT EXISTS customer_contacts(customer_id VARCHAR(256), channel_type VARCHAR(256), channel_id VARCHAR(256), PRIMARY KEY(channel_type, channel_id), INDEX index_cc1 (customer_id(255), channel_type))`
	queryConversationsTable := `CREATE TABLE IF NOT EXISTS conversations(id VARCHAR(256) primary key, type INT, customer_id TEXT, connected_agent TEXT, created_timestamp BIGINT, state INT, INDEX index_c1 (customer_id(255), state))`
	queryMessagesTable := `CREATE TABLE IF NOT EXISTS messages(id INT primary key auto_increment, conversation_id VARCHAR(256), body TEXT, timestamp BIGINT, status INT, sent_from_agent BOOLEAN, type INT, event TEXT, INDEX index_m1 (conversation_id, status, sent_from_agent))`

	DBConnector.CreateTable(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, queryCustomersTable)
	DBConnector.CreateTable(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, queryCustomerContactsTable)
	DBConnector.CreateTable(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, queryConversationsTable)
	DBConnector.CreateTable(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, queryMessagesTable)

	o.InitializeChannels()
}

func (o *OmniChannel) InitializeChannels() {

	o.Channels = make(map[types.ChannelType]Channel)
	o.Channels[types.Viber] = Viber{}
	o.Channels[types.WhatsApp] = WhatsApp{}

	for index, channel := range o.Channels {
		channel = o.Channels[index]
		channel.Init()
	}
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

	defer MeasureExecutionTime()()

	channelType := Omnichannel.getChannelType(req.URL.String())
	if channelType == types.Unknown {
		return
	}

	channel := Omnichannel.Channels[channelType]

	event, data := channel.parseReceivedData(body)

	senderUniqueID, senderName := channel.getSenderInfo(data)

	var customerID string
	if customerID = Omnichannel.FindCustomer(channelType, senderUniqueID); customerID == "" {
		customer := Omnichannel.FindCustomerByName(senderName)
		if customer == nil {
			customer = &types.Customer{Id: uuid.New().String(), Name: senderName}
			Omnichannel.AddNewCustomer(customer, channelType, senderUniqueID)
		} else {
			Omnichannel.AddNewCustomerContact(customer, channelType, senderUniqueID)
		}
	}

	if event == EVENT_NEW_MESSAGE {

		messageText, timestamp := channel.getMessageInfo(data)
		message := types.Message{Type: types.Text, Text: messageText, Timestamp: timestamp, Status: types.Sent, SentFromAgent: false}

		var conversation *types.Conversation = nil
		if conversation = Omnichannel.FindActiveConversationFromCustomer(customerID); conversation == nil {

			conversation = &types.Conversation{Id: uuid.New().String(), Type: channelType, State: types.Unassigned, CustomerID: customerID, Created_Timestamp: timestamp}

			Omnichannel.AddNewConversation(conversation)
			Omnichannel.AddNewMessage(conversation.Id, Omnichannel.createEventMessage(EVENT_CONVERSATION_STARTED, timestamp))
			Omnichannel.AddNewMessage(conversation.Id, message)

			channel.sendMessage(senderUniqueID, "Thank you for contacting us. One of agents will answer as soon as possible.", true)
			Omnichannel.SendNewConversationToAgents(*conversation)
		} else {

			Omnichannel.AddNewMessage(conversation.Id, message)

			jsonData := make(map[string]interface{})
			jsonData["event"] = event
			jsonData["conversationID"] = conversation.Id
			jsonData["message"] = message

			TcpServer.SendEventToAgents(jsonData, conversation.ConnectedAgent)
		}

	} else if event == EVENT_MESSAGE_STATUS_UPDATED {

		status := channel.getMessageStatus(data)

		if conversation := Omnichannel.FindActiveConversationFromCustomer(customerID); conversation != nil {

			Omnichannel.UpdateMessageStatus(conversation.Id, status)

			jsonData := make(map[string]interface{})
			jsonData["event"] = event
			jsonData["conversationID"] = conversation.Id
			jsonData["status"] = status
			TcpServer.SendEventToAgents(jsonData, "")
		}
	}
}

func MeasureExecutionTime() func() {
	startTime := time.Now()
	return func() {
		executionTime += time.Since(startTime).Seconds()
		fmt.Printf("Total Elapsed time is %f \n", executionTime)
	}
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

func (o *OmniChannel) sendMessage(conversationID string, messageTxt string, autoreply bool) {

	if conversation := o.FindConversationByID(conversationID); conversation != nil {

		customerUniqueID := o.FindCustomerUniqueIdByChannel(conversation.CustomerID, conversation.Type)
		channel := o.Channels[conversation.Type]
		channel.sendMessage(customerUniqueID, messageTxt, autoreply)

		if !autoreply {
			message := types.Message{Type: types.Text, Text: messageTxt, Timestamp: uint(time.Now().UnixMilli()), Status: types.Sent, SentFromAgent: true}
			o.AddNewMessage(conversationID, message)
		}
	}
}

func (o *OmniChannel) SendNewConversationToAgents(conversation types.Conversation) {

	jsonData := make(map[string]interface{})
	jsonData["event"] = EVENT_NEW_CONVERSATION
	jsonData["conversationID"] = conversation.Id
	jsonData["type"] = conversation.Type
	jsonData["customer"] = o.FindCustomerByID(conversation.CustomerID)

	TcpServer.SendEventToAgents(jsonData, "")
}

func (o *OmniChannel) FinishConversation(conversationID string) {
	o.AddNewMessage(conversationID, Omnichannel.createEventMessage(EVENT_CONVERSATION_FINISHED, uint(time.Now().UnixMilli())))
	o.UpdateConversationState(conversationID, types.Finished, "")
}

func (o *OmniChannel) AcceptConversation(conversationID string, agentExt string) {
	o.UpdateConversationState(conversationID, types.Assigned, agentExt)
}

func (o *OmniChannel) createEventMessage(event string, timestamp uint) types.Message {

	message := types.Message{Type: types.Event, Event: event, Timestamp: timestamp, Status: types.Sent, SentFromAgent: false}
	return message
}

func (o *OmniChannel) FindCustomer(channelType types.ChannelType, channelID string) string {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT customer_id FROM customer_contacts WHERE channel_type="+strconv.Itoa(int(channelType))+" AND channel_id='"+channelID+"'")
	defer results.Close()

	if results.Next() {
		var customer_id string
		if err := results.Scan(&customer_id); err == nil {
			return customer_id
		}
	}

	return ""
}

func (o *OmniChannel) FindCustomerByID(customer_id string) *types.Customer {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT name FROM customer WHERE id='"+customer_id+"'")
	defer results.Close()

	if results.Next() {
		contact := types.Customer{}
		contact.Id = customer_id
		if err := results.Scan(&contact.Name); err == nil {
			return &contact
		}
	}

	return nil
}

func (o *OmniChannel) FindCustomerByName(name string) *types.Customer {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT id, name FROM customer WHERE name='"+name+"'")
	defer results.Close()

	if results.Next() {
		contact := types.Customer{}
		if err := results.Scan(&contact.Id, &contact.Name); err == nil {
			return &contact
		}
	}

	return nil
}

func (o *OmniChannel) FindCustomerUniqueIdByChannel(customerID string, channelType types.ChannelType) string {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT channel_id FROM customer_contacts WHERE customer_id='"+customerID+"' AND channel_type="+strconv.Itoa(int(channelType)))
	defer results.Close()

	if results.Next() {
		var channelID string
		if err := results.Scan(&channelID); err == nil {
			return channelID
		}
	}

	return ""
}

func (o *OmniChannel) AddNewCustomer(customer *types.Customer, channelType types.ChannelType, channelID string) {
	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "INSERT INTO customer(id, name) VALUES('"+customer.Id+"','"+customer.Name+"')")
	defer results.Close()

	results2 := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "INSERT INTO customer_contacts(customer_id, channel_type, channel_id) VALUES('"+customer.Id+"',"+strconv.Itoa(int(channelType))+",'"+channelID+"')")
	defer results2.Close()
}

func (o *OmniChannel) AddNewCustomerContact(customer *types.Customer, channelType types.ChannelType, channelID string) {
	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "INSERT INTO customer_contacts(customer_id, channel_type, channel_id) VALUES('"+customer.Id+"',"+strconv.Itoa(int(channelType))+",'"+channelID+"')")
	defer results.Close()
}

func (o *OmniChannel) FindActiveConversationFromCustomer(customerID string) *types.Conversation {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT id, type, connected_agent, created_timestamp, state FROM conversations WHERE customer_id='"+customerID+"' AND state<"+strconv.Itoa(int(types.Finished)))
	defer results.Close()

	if results.Next() {
		conversation := types.Conversation{}
		conversation.CustomerID = customerID
		if err := results.Scan(&conversation.Id, &conversation.Type, &conversation.ConnectedAgent, &conversation.Created_Timestamp, &conversation.State); err == nil {
			return &conversation
		}
	}

	return nil
}

func (o *OmniChannel) FindConversationByID(conversationID string) *types.Conversation {

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT customer_id, type, connected_agent, created_timestamp, state FROM conversations WHERE id='"+conversationID+"'")
	defer results.Close()

	if results.Next() {
		conversation := types.Conversation{}
		conversation.Id = conversationID
		if err := results.Scan(&conversation.CustomerID, &conversation.Type, &conversation.ConnectedAgent, &conversation.Created_Timestamp, &conversation.State); err == nil {
			return &conversation
		}
	}

	return nil
}

func (o *OmniChannel) AddNewConversation(conversation *types.Conversation) {

	query := "INSERT INTO conversations(id, type, customer_id, connected_agent, created_timestamp, state) VALUES('" +
		conversation.Id + "'," +
		"'" + strconv.Itoa(int(conversation.Type)) + "'," +
		"'" + conversation.CustomerID + "'," +
		"'" + conversation.ConnectedAgent + "'," +
		strconv.FormatUint(uint64(conversation.Created_Timestamp), 10) + "," +
		strconv.Itoa(int(conversation.State)) + ")"

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, query)
	defer results.Close()
}

func (o *OmniChannel) UpdateConversationState(conversationID string, state types.ConversationState, connectedAgent string) {

	var query string

	if connectedAgent == "" {
		query = "UPDATE conversations SET state=" + strconv.Itoa(int(state)) + " WHERE id='" + conversationID + "'"
	} else {
		query = "UPDATE conversations SET state=" + strconv.Itoa(int(state)) + ", connectedAgent='" + connectedAgent + "' WHERE id='" + conversationID + "'"
	}

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, query)
	defer results.Close()
}

func (o *OmniChannel) AddNewMessage(conversationID string, message types.Message) {

	query := "INSERT INTO messages(conversation_id, body, timestamp, status, sent_from_agent, type, event) VALUES('" +
		conversationID + "'," +
		"'" + message.Text + "'," +
		strconv.FormatUint(uint64(message.Timestamp), 10) + "," +
		strconv.Itoa(int(message.Status)) + "," +
		strconv.FormatBool(message.SentFromAgent) + "," +
		strconv.Itoa(int(message.Type)) + "," +
		"'" + message.Event + "')"

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, query)
	defer results.Close()
}

func (o *OmniChannel) UpdateMessageStatus(conversationID string, status types.MessageStatus) {
	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "UPDATE messages SET status="+strconv.Itoa(int(status))+" WHERE conversation_id='"+conversationID+"' AND status<"+strconv.Itoa(int(status))+" AND sent_from_agent=true")
	defer results.Close()
}

func (o *OmniChannel) GetMessages(conversationID string) []*types.Message {

	var messages []*types.Message

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT body, timestamp, status, sent_from_agent, type, event FROM messages WHERE conversation_id='"+conversationID+"'")
	defer results.Close()

	for results.Next() {
		var message types.Message
		if err := results.Scan(&message.Text, &message.Timestamp, &message.Status, &message.SentFromAgent, &message.Type, &message.Event); err == nil {
			messages = append(messages, &message)
		}
	}

	return messages
}

func (o *OmniChannel) GetCustomerConversations(customerID string) []*types.Conversation {

	var conversations []*types.Conversation

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT id, type, customer_id, connected_agent, created_timestamp FROM conversations WHERE customer_id='"+customerID+"' AND state="+strconv.Itoa(int(types.Finished)))
	defer results.Close()

	for results.Next() {
		var conversation types.Conversation
		if err := results.Scan(&conversation.Id, &conversation.Type, &conversation.CustomerID, &conversation.ConnectedAgent, &conversation.Created_Timestamp); err == nil {
			conversations = append(conversations, &conversation)
		}
	}

	return conversations
}

func (o *OmniChannel) GetAgentActiveConversations(agent string) []*types.Conversation {

	var conversations []*types.Conversation

	results := DBConnector.ExecuteQuery(OMNI_DB_CREDENTIALS, OMNI_DB_NAME, "SELECT id, type, customer_id, connected_agent, created_timestamp, state FROM conversations WHERE connected_agent='"+agent+"' AND state<"+strconv.Itoa(int(types.Finished)))
	defer results.Close()

	for results.Next() {
		var conversation types.Conversation
		if err := results.Scan(&conversation.Id, &conversation.Type, &conversation.CustomerID, &conversation.ConnectedAgent, &conversation.Created_Timestamp, &conversation.State); err == nil {
			conversations = append(conversations, &conversation)
		}
	}

	return conversations
}
