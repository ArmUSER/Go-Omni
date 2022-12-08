package services

import (
	"strconv"
)

var CallHistory CALLHISTORY

type CALLHISTORY struct{}

func (c *CALLHISTORY) GetCallsByContactID(id string) []*Call {

	var calls []*Call

	results := DBConnector.ExecuteQuery(OMNI_DB, "SELECT id, connectedAgent, timestamp,  duration FROM calls WHERE contactID='"+id+"'")
	for results.Next() {
		var call Call
		if err := results.Scan(&call.CallId, &call.CalleeNumber, &call.Timestamp, &call.Duration); err == nil {
			calls = append(calls, &call)
		}
	}

	return calls
}

func (c *CALLHISTORY) AddCallToDB(call *Call) {

	query := "INSERT INTO calls(id, contactID, connectedAgent, timestamp, duration) VALUES(" +
		"'" + call.CallId + "'," +
		"'" + call.ContactID + "'," +
		"'" + call.CalleeNumber + "'," +
		strconv.FormatUint(uint64(call.Timestamp), 10) + "," +
		strconv.Itoa(call.Duration) + ")"

	DBConnector.ExecuteQuery(OMNI_DB, query)
}

func (c *CALLHISTORY) UpdateCallContactID(currentID string, updatedID string) {
	DBConnector.ExecuteQuery(OMNI_DB, "UPDATE calls SET contactID='"+updatedID+"' WHERE contactID='"+currentID+"'")
}
