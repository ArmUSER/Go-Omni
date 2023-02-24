package types

import "net"

type ChannelType int

const (
	Viber ChannelType = iota
	WhatsApp
	Unknown
)

type ConversationState int

const (
	Unassigned ConversationState = iota
	Assigned
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

type Customer struct {
	Id   string
	Name string
}

type Conversation struct {
	Id                string
	Type              ChannelType
	State             ConversationState
	CustomerID        string
	ConnectedAgent    string
	Messages          []Message
	Created_Timestamp uint
}

type Agent struct {
	Id            string
	Name          string
	Conversations int
	Socket        net.Conn
}
