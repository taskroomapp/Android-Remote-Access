package models

import (
	"time"

	"github.com/google/uuid"
)

// StoredContact is a persisted device contact row.
// Columns: DisplayName, Number, DataEntryDate
type StoredContact struct {
	ID            uuid.UUID `json:"id"`
	DeviceID      uuid.UUID `json:"device_id"`
	NativeID      string    `json:"native_id,omitempty"`
	DisplayName   string    `json:"display_name"`
	Number        string    `json:"number"`
	NumberFP      string    `json:"-"`
	DataEntryDate time.Time `json:"data_entry_date"`
}

// StoredSMS is a persisted SMS/MMS message row.
// Columns: Id, isRead, Address, Message, Name, Person, Date, MessageType, DataEntryDate
type StoredSMS struct {
	ID            uuid.UUID  `json:"id"`
	DeviceID      uuid.UUID  `json:"device_id"`
	NativeID      string     `json:"native_id,omitempty"`
	IsRead        bool       `json:"is_read"`
	Address       string     `json:"address"`
	Message       string     `json:"message"`
	Name          string     `json:"name"`
	Person        string     `json:"person"`
	MessageDate   *time.Time `json:"message_date,omitempty"`
	MessageType   string     `json:"message_type"`
	DataEntryDate time.Time  `json:"data_entry_date"`
}

// StoredCallLog is a persisted call-log row.
// Columns: CallID, Number, NameCall, Duration, TypeCall, DateCall, IDContacts, DataEntryDate
type StoredCallLog struct {
	ID            uuid.UUID  `json:"id"`
	DeviceID      uuid.UUID  `json:"device_id"`
	CallID        string     `json:"call_id"`
	Number        string     `json:"number"`
	NameCall      string     `json:"name_call"`
	Duration      int        `json:"duration"`
	TypeCall      string     `json:"type_call"`
	DateCall      *time.Time `json:"date_call,omitempty"`
	IDContacts    string     `json:"id_contacts"`
	DataEntryDate time.Time  `json:"data_entry_date"`
}

// CommsSaveRequest is the body for persisting panel/device communications data.
type CommsSaveRequest struct {
	Contacts []map[string]interface{} `json:"contacts,omitempty"`
	Messages []map[string]interface{} `json:"messages,omitempty"`
	Calls    []map[string]interface{} `json:"calls,omitempty"`
}

// CommsSaveResult reports how many rows were upserted.
type CommsSaveResult struct {
	ContactsSaved int `json:"contacts_saved"`
	MessagesSaved int `json:"messages_saved"`
	CallsSaved    int `json:"calls_saved"`
}
