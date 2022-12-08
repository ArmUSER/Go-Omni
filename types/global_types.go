package types

type ChannelType int

const (
	Viber ChannelType = iota
	WhatsApp
	PhoneCall
	Unknown
)

type ConversationState int

const (
	Default ConversationState = iota
	InQueue
	ConnectedToAgent
	Finished
)

type MessageStatus int

const (
	Sent MessageStatus = iota
	Delivered
	Seen
)

type MessageType int

const (
	Text MessageType = iota
	Event
)

type Message struct {
	Text          string
	Timestamp     uint
	Status        MessageStatus
	SentFromAgent bool
	Type          MessageType
	Event         string
}

type Contact struct {
	Id      string
	Number  string
	Name    string
	ViberId string
}

type Conversation struct {
	Id                string
	Type              ChannelType
	State             ConversationState
	Customer          Contact
	ConnectedAgent    string
	Messages          []Message
	Created_Timestamp uint
}
