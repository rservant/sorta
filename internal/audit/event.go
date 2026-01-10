// Package audit provides audit trail functionality for Sorta file operations.
package audit

import (
	"encoding/json"
	"time"
)

// ISO8601Format is the time format used for audit event timestamps.
const ISO8601Format = time.RFC3339

// eventJSON is the internal representation for JSON marshaling/unmarshaling.
// It uses pointers for optional fields to properly handle omitempty.
type eventJSON struct {
	Timestamp       string            `json:"timestamp"`
	RunID           RunID             `json:"runId"`
	EventType       EventType         `json:"eventType"`
	Status          OperationStatus   `json:"status"`
	SourcePath      *string           `json:"sourcePath,omitempty"`
	DestinationPath *string           `json:"destinationPath,omitempty"`
	ReasonCode      *ReasonCode       `json:"reasonCode,omitempty"`
	FileIdentity    *FileIdentity     `json:"fileIdentity,omitempty"`
	ErrorDetails    *ErrorDetails     `json:"errorDetails,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler for AuditEvent.
// It ensures timestamps are in ISO 8601 format and optional fields are omitted when empty.
// Requirements: 3.1, 8.2, 8.3
func (e AuditEvent) MarshalJSON() ([]byte, error) {
	ej := eventJSON{
		Timestamp:    e.Timestamp.Format(ISO8601Format),
		RunID:        e.RunID,
		EventType:    e.EventType,
		Status:       e.Status,
		FileIdentity: e.FileIdentity,
		ErrorDetails: e.ErrorDetails,
		Metadata:     e.Metadata,
	}

	// Only include optional string fields if non-empty
	if e.SourcePath != "" {
		ej.SourcePath = &e.SourcePath
	}
	if e.DestinationPath != "" {
		ej.DestinationPath = &e.DestinationPath
	}
	if e.ReasonCode != "" {
		rc := e.ReasonCode
		ej.ReasonCode = &rc
	}

	return json.Marshal(ej)
}

// UnmarshalJSON implements json.Unmarshaler for AuditEvent.
// It parses ISO 8601 timestamps and handles optional fields.
// Requirements: 3.1, 8.2, 8.3
func (e *AuditEvent) UnmarshalJSON(data []byte) error {
	var ej eventJSON
	if err := json.Unmarshal(data, &ej); err != nil {
		return err
	}

	// Parse timestamp
	t, err := time.Parse(ISO8601Format, ej.Timestamp)
	if err != nil {
		return err
	}

	e.Timestamp = t
	e.RunID = ej.RunID
	e.EventType = ej.EventType
	e.Status = ej.Status
	e.FileIdentity = ej.FileIdentity
	e.ErrorDetails = ej.ErrorDetails
	e.Metadata = ej.Metadata

	// Handle optional string fields
	if ej.SourcePath != nil {
		e.SourcePath = *ej.SourcePath
	}
	if ej.DestinationPath != nil {
		e.DestinationPath = *ej.DestinationPath
	}
	if ej.ReasonCode != nil {
		e.ReasonCode = *ej.ReasonCode
	}

	return nil
}

// MarshalJSONLine marshals an AuditEvent to a JSON line (no trailing newline).
// This is used for JSON Lines format output.
func (e AuditEvent) MarshalJSONLine() ([]byte, error) {
	return e.MarshalJSON()
}

// UnmarshalJSONLine unmarshals a JSON line into an AuditEvent.
// This is used for JSON Lines format input.
func UnmarshalJSONLine(data []byte) (*AuditEvent, error) {
	var e AuditEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}
