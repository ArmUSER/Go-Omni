package channels

import (
	"server/types"
)

type Channel interface {
	Init()
	ParseReceivedData(body []byte) (event string, data map[string]interface{})
	GetSenderInfo(data map[string]interface{}) (senderUniqueID string, senderName string)
	GetMessageInfo(data map[string]interface{}) (messageText string, msgTimestamp uint)
	GetMessageStatus(data map[string]interface{}) types.MessageStatus
	SendMessage(senderUniqueID string, messageText string, autoreply bool)
}
