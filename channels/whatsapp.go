package channels

import (
	"log"
	"net/http"
	"net/url"
	"server/types"
	"strings"
	"time"

	"github.com/go-ini/ini"
)

var (
	WHATSAPP_SID        string
	WHATSAPP_AUTH_TOKEN string
	WHATSAPP_NUMBER     string
)

type WhatsApp struct{}

func (v WhatsApp) Init() {
	cfg, err := ini.Load("conf/channels_conf.ini")
	if err != nil {
		log.Println("Failed to read conf file: ", err)
	}

	WHATSAPP_SID = cfg.Section("whatsapp").Key("sid").String()
	WHATSAPP_AUTH_TOKEN = cfg.Section("whatsapp").Key("token").String()
	WHATSAPP_NUMBER = cfg.Section("whatsapp").Key("number").String()
}

func (v WhatsApp) ParseReceivedData(body []byte) (string, map[string]interface{}) {

	data := make(map[string]interface{})
	var event string

	if params, err := url.ParseQuery(string(body)); err == nil {
		for key, value := range params {
			data[key] = value
		}
	}

	if data["MessageStatus"] == nil {
		event = types.EVENT_NEW_MESSAGE
	} else {
		event = types.EVENT_MESSAGE_STATUS_UPDATED
	}

	return event, data
}

func (v WhatsApp) GetSenderInfo(data map[string]interface{}) (string, string) {

	var senderUniqueID string
	var senderName string

	var number string
	from := []string{}
	receiver := []string{}
	var profileInfo []string = nil

	if data["From"] != nil {
		from = data["From"].([]string)
	}

	if data["To"] != nil {
		receiver = data["To"].([]string)
	}

	if from != nil && from[0] != "whatsapp:"+WHATSAPP_NUMBER {
		number = from[0]
	} else {
		number = receiver[0]
	}

	senderUniqueID = strings.ReplaceAll(number, "whatsapp:+387", "0")

	if data["ProfileName"] != nil {
		profileInfo = data["ProfileName"].([]string)
	}

	if profileInfo != nil {
		senderName = profileInfo[0]
	}

	return senderUniqueID, senderName
}

func (v WhatsApp) GetMessageInfo(data map[string]interface{}) (string, uint) {

	var (
		messageText string
		timestamp   uint
	)

	body := data["Body"].([]string)

	messageText = body[0]
	timestamp = uint(time.Now().UnixMilli())

	return messageText, timestamp
}

func (v WhatsApp) GetMessageStatus(data map[string]interface{}) types.MessageStatus {

	var status types.MessageStatus

	event := data["MessageStatus"].([]string)[0]

	if event == "delivered" {
		status = types.Delivered
	} else if event == "read" {
		status = types.Seen
	}

	return status
}

func (w WhatsApp) SendMessage(customerID string, messageText string, autoreply bool) {

	urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + WHATSAPP_SID + "/Messages.json"

	v := url.Values{}
	v.Set("To", "whatsapp:+387"+customerID[1:])
	v.Set("From", "whatsapp:"+WHATSAPP_NUMBER)
	v.Set("Body", messageText)
	rb := *strings.NewReader(v.Encode())

	req, _ := http.NewRequest("POST", urlStr, &rb)
	req.SetBasicAuth(WHATSAPP_SID, WHATSAPP_AUTH_TOKEN)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Failed to send whatsapp message", err)
	}

	//outputAsBytes, _ := ioutil.ReadAll(response.Body)
	//log.Println(string(outputAsBytes))

	defer response.Body.Close()
}
