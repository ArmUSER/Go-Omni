package services

import (
	"server/types"
)

var ContactDirectory CONTACTDIRECTORY

type CONTACTDIRECTORY struct{}

func (c *CONTACTDIRECTORY) FindContactByID(id string) *types.Contact {

	results := DBConnector.ExecuteQuery(OMNI_DB, "SELECT id, name, number, viberID FROM contacts WHERE id='"+id+"'")
	if results.Next() {
		contact := types.Contact{}

		if err := results.Scan(&contact.Id, &contact.Name, &contact.Number, &contact.ViberId); err == nil {
			return &contact
		}
	}

	return nil
}

func (c *CONTACTDIRECTORY) FindContact(field string, value string) types.Contact {

	results := DBConnector.ExecuteQuery(OMNI_DB, "SELECT id, name, number, viberID FROM contacts WHERE "+field+"='"+value+"'")
	if results.Next() {
		contact := types.Contact{}

		if err := results.Scan(&contact.Id, &contact.Name, &contact.Number, &contact.ViberId); err == nil {
			return contact
		}
	}

	return types.Contact{}
}

func (c *CONTACTDIRECTORY) AddContact(contact types.Contact) {
	DBConnector.ExecuteQuery(OMNI_DB, "INSERT INTO contacts(id, name, number, viberID) VALUES('"+contact.Id+"','"+contact.Name+"','"+contact.Number+"','"+contact.ViberId+"')")
}

func (c *CONTACTDIRECTORY) UpdateContact(contactID string, field string, value string) {
	DBConnector.ExecuteQuery(OMNI_DB, "UPDATE contacts SET "+field+"='"+value+"' WHERE id='"+contactID+"'")
}

func (c *CONTACTDIRECTORY) DeleteContact(id string) {
	DBConnector.ExecuteQuery(OMNI_DB, "DELETE FROM contacts WHERE id='"+id+"'")
}
