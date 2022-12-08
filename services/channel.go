package services

import (
	"server/types"
)

type Channel interface {
	Init()
	parseReceivedData(body []byte) (string, map[string]interface{}) // returns event, and parsed data that is passed in the below functions
	getSenderInfo(data map[string]interface{}) (senderUniqueID string, senderName string)
	getMessageInfo(data map[string]interface{}) (messageText string, msgTimestamp uint)
	getMessageStatus(data map[string]interface{}) types.MessageStatus
	sendMessage(customer types.Contact, messageText string, autoreply bool)

	findContact(senderID string) types.Contact
	createContact(senderID string, senderName string) types.Contact
}
