package services

import (
	"server/types"
)

type Channel interface {
	Init()
	parseReceivedData(body []byte) (event string, data map[string]interface{})
	getSenderInfo(data map[string]interface{}) (senderUniqueID string, senderName string)
	getMessageInfo(data map[string]interface{}) (messageText string, msgTimestamp uint)
	getMessageStatus(data map[string]interface{}) types.MessageStatus
	sendMessage(senderUniqueID string, messageText string, autoreply bool)
}
