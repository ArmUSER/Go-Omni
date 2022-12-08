package services

import (
	"fmt"
	"server/types"
	"strconv"
)

var ConversationHistory CONVERSATIONHISTORY

type CONVERSATIONHISTORY struct{}

func (c *CONVERSATIONHISTORY) GetConversationsByContactID(contactID string) []*types.Conversation {

	var conversations []*types.Conversation

	results := DBConnector.ExecuteQuery(OMNI_DB, "SELECT id, type, contactID, connectedAgent, created_timestamp FROM conversations WHERE contactID='"+contactID+"'")
	for results.Next() {
		var conversation types.Conversation
		if err := results.Scan(&conversation.Id, &conversation.Type, &conversation.Customer.Id, &conversation.ConnectedAgent, &conversation.Created_Timestamp); err == nil {
			results2 := DBConnector.ExecuteQuery(OMNI_DB, "SELECT body, timestamp, status, sentFromAgent, type, event FROM messages WHERE conversation_id='"+conversation.Id+"'")
			for results2.Next() {
				var message types.Message
				if err = results2.Scan(&message.Text, &message.Timestamp, &message.Status, &message.SentFromAgent, &message.Type, &message.Event); err == nil {
					conversation.Messages = append(conversation.Messages, message)
				}
			}

			conversations = append(conversations, &conversation)
			fmt.Println("conversation from db ", conversation.Id, conversation.Type, conversation.Customer.Id, conversation.ConnectedAgent, conversation.Messages)
		}
	}

	return conversations
}

func (c *CONVERSATIONHISTORY) AddConversationToDB(conversation *types.Conversation) {

	query := "INSERT INTO conversations(id, type, contactID, connectedAgent, created_timestamp, state) VALUES('" +
		conversation.Id + "'," +
		"'" + strconv.Itoa(int(conversation.Type)) + "'," +
		"'" + conversation.Customer.Id + "'," +
		"'" + conversation.ConnectedAgent + "'," +
		strconv.FormatUint(uint64(conversation.Created_Timestamp), 10) + "," +
		strconv.Itoa(int(conversation.State)) + ")"

	DBConnector.ExecuteQuery(OMNI_DB, query)
}

func (c *CONVERSATIONHISTORY) AddMessageToDB(conversation_id string, message types.Message) {

	query := "INSERT INTO messages(conversation_id, body, timestamp, status, sentFromAgent, type, event) VALUES('" +
		conversation_id + "'," +
		"'" + message.Text + "'," +
		strconv.FormatUint(uint64(message.Timestamp), 10) + "," +
		strconv.Itoa(int(message.Status)) + "," +
		strconv.FormatBool(message.SentFromAgent) + "," +
		strconv.Itoa(int(message.Type)) + "," +
		"'" + message.Event + "')"

	DBConnector.ExecuteQuery(OMNI_DB, query)
}

func (c *CONVERSATIONHISTORY) UpdateConversationContactID(currentID string, updatedID string) {
	DBConnector.ExecuteQuery(OMNI_DB, "UPDATE conversations SET contactID='"+updatedID+"' WHERE contactID='"+currentID+"'")
}

func (c *CONVERSATIONHISTORY) UpdateConversationStateInDB(conversationID string, state types.ConversationState, connectedAgent string) {

	var query string

	if connectedAgent == "" {
		query = "UPDATE conversations SET state=" + strconv.Itoa(int(state)) + " WHERE id='" + conversationID + "'"
	} else {
		query = "UPDATE conversations SET state=" + strconv.Itoa(int(state)) + ", connectedAgent='" + connectedAgent + "' WHERE id='" + conversationID + "'"
	}

	DBConnector.ExecuteQuery(OMNI_DB, query)
}
