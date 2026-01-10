package audit

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: audit-trail, Property 11: JSON Lines Round-Trip
// Validates: Requirements 8.2, 8.3

// genRunID generates valid UUID v4 format RunIDs.
func genRunID() gopter.Gen {
	return gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) >= 8
	}).Map(func(s string) RunID {
		// Generate a UUID-like string
		return RunID("test-" + s[:8] + "-run")
	})
}

// genEventType generates valid EventType values.
func genEventType() gopter.Gen {
	eventTypes := []EventType{
		EventRunStart, EventRunEnd,
		EventMove, EventRouteToReview, EventSkip,
		EventDuplicateDetected, EventParseFailure, EventValidationFailure, EventError,
		EventUndoMove, EventUndoSkip, EventIdentityMismatch, EventAmbiguousIdentity,
		EventCollision, EventSourceMissing, EventContentChanged, EventConflictDetected,
		EventRotation, EventRetentionPrune, EventLogInitialized,
	}
	return gen.IntRange(0, len(eventTypes)-1).Map(func(i int) EventType {
		return eventTypes[i]
	})
}

// genOperationStatus generates valid OperationStatus values.
func genOperationStatus() gopter.Gen {
	statuses := []OperationStatus{StatusSuccess, StatusFailure, StatusSkipped}
	return gen.IntRange(0, len(statuses)-1).Map(func(i int) OperationStatus {
		return statuses[i]
	})
}

// genReasonCode generates valid ReasonCode values (including empty).
func genReasonCode() gopter.Gen {
	reasons := []ReasonCode{
		"", // empty is valid
		ReasonNoMatch, ReasonInvalidDate, ReasonAlreadyProcessed,
		ReasonUnclassified, ReasonParseError, ReasonValidationError,
		ReasonDuplicateRenamed, ReasonNoOpEvent, ReasonIdentityMismatch,
		ReasonDestinationOccupied, ReasonSourceNotFound, ReasonConflictWithLaterRun,
	}
	return gen.IntRange(0, len(reasons)-1).Map(func(i int) ReasonCode {
		return reasons[i]
	})
}

// genTimestamp generates valid timestamps.
func genTimestamp() gopter.Gen {
	return gen.Int64Range(0, 2000000000).Map(func(unix int64) time.Time {
		return time.Unix(unix, 0).UTC()
	})
}

// genOptionalString generates an optional string (empty or non-empty).
func genOptionalString() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(""),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) <= 100
		}),
	)
}

// genHexString generates a 64-character hex string (SHA-256 hash format).
func genHexString() gopter.Gen {
	hexChars := []byte("0123456789abcdef")
	return gen.SliceOfN(64, gen.IntRange(0, 15)).Map(func(indices []int) string {
		result := make([]byte, 64)
		for i, idx := range indices {
			result[i] = hexChars[idx]
		}
		return string(result)
	})
}

// genFileIdentity generates optional FileIdentity.
func genFileIdentity() gopter.Gen {
	return gen.OneGenOf(
		gen.Const((*FileIdentity)(nil)),
		gopter.CombineGens(
			genHexString(),
			gen.Int64Range(0, 1000000000),
			genTimestamp(),
		).Map(func(vals []interface{}) *FileIdentity {
			return &FileIdentity{
				ContentHash: vals[0].(string),
				Size:        vals[1].(int64),
				ModTime:     vals[2].(time.Time),
			}
		}),
	)
}

// genErrorDetails generates optional ErrorDetails.
func genErrorDetails() gopter.Gen {
	return gen.OneGenOf(
		gen.Const((*ErrorDetails)(nil)),
		gen.Struct(reflect.TypeOf(ErrorDetails{}), map[string]gopter.Gen{
			"ErrorType":    gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 50 }),
			"ErrorMessage": gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 200 }),
			"Operation":    gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 50 }),
		}).Map(func(ed ErrorDetails) *ErrorDetails {
			return &ed
		}),
	)
}

// genMetadata generates optional metadata map.
func genMetadata() gopter.Gen {
	return gen.OneGenOf(
		gen.Const((map[string]string)(nil)),
		gen.MapOf(
			gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 20 }),
			gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 100 }),
		).SuchThat(func(m map[string]string) bool {
			return len(m) <= 5
		}),
	)
}

// genAuditEvent generates valid AuditEvent instances.
func genAuditEvent() gopter.Gen {
	return gopter.CombineGens(
		genTimestamp(),
		genRunID(),
		genEventType(),
		genOperationStatus(),
		genOptionalString(), // sourcePath
		genOptionalString(), // destinationPath
		genReasonCode(),
		genFileIdentity(),
		genErrorDetails(),
		genMetadata(),
	).Map(func(vals []interface{}) AuditEvent {
		return AuditEvent{
			Timestamp:       vals[0].(time.Time),
			RunID:           vals[1].(RunID),
			EventType:       vals[2].(EventType),
			Status:          vals[3].(OperationStatus),
			SourcePath:      vals[4].(string),
			DestinationPath: vals[5].(string),
			ReasonCode:      vals[6].(ReasonCode),
			FileIdentity:    vals[7].(*FileIdentity),
			ErrorDetails:    vals[8].(*ErrorDetails),
			Metadata:        vals[9].(map[string]string),
		}
	})
}

// TestJSONLinesRoundTrip tests Property 11: JSON Lines Round-Trip
// For any valid AuditEvent, serializing to JSON Lines format and deserializing
// SHALL produce an equivalent event object.
func TestJSONLinesRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("JSON Lines round-trip preserves AuditEvent", prop.ForAll(
		func(event AuditEvent) bool {
			// Marshal to JSON
			data, err := event.MarshalJSON()
			if err != nil {
				t.Logf("MarshalJSON failed: %v", err)
				return false
			}

			// Unmarshal back
			var restored AuditEvent
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Logf("UnmarshalJSON failed: %v", err)
				return false
			}

			// Compare timestamps (truncate to second precision due to JSON format)
			if !event.Timestamp.Truncate(time.Second).Equal(restored.Timestamp.Truncate(time.Second)) {
				t.Logf("Timestamp mismatch: %v vs %v", event.Timestamp, restored.Timestamp)
				return false
			}

			// Compare required fields
			if event.RunID != restored.RunID {
				t.Logf("RunID mismatch: %v vs %v", event.RunID, restored.RunID)
				return false
			}
			if event.EventType != restored.EventType {
				t.Logf("EventType mismatch: %v vs %v", event.EventType, restored.EventType)
				return false
			}
			if event.Status != restored.Status {
				t.Logf("Status mismatch: %v vs %v", event.Status, restored.Status)
				return false
			}

			// Compare optional string fields
			if event.SourcePath != restored.SourcePath {
				t.Logf("SourcePath mismatch: %v vs %v", event.SourcePath, restored.SourcePath)
				return false
			}
			if event.DestinationPath != restored.DestinationPath {
				t.Logf("DestinationPath mismatch: %v vs %v", event.DestinationPath, restored.DestinationPath)
				return false
			}
			if event.ReasonCode != restored.ReasonCode {
				t.Logf("ReasonCode mismatch: %v vs %v", event.ReasonCode, restored.ReasonCode)
				return false
			}

			// Compare FileIdentity
			if !compareFileIdentity(event.FileIdentity, restored.FileIdentity) {
				t.Logf("FileIdentity mismatch")
				return false
			}

			// Compare ErrorDetails
			if !compareErrorDetails(event.ErrorDetails, restored.ErrorDetails) {
				t.Logf("ErrorDetails mismatch")
				return false
			}

			// Compare Metadata
			if !compareMetadata(event.Metadata, restored.Metadata) {
				t.Logf("Metadata mismatch")
				return false
			}

			return true
		},
		genAuditEvent(),
	))

	properties.TestingRun(t)
}

// compareFileIdentity compares two FileIdentity pointers for equality.
func compareFileIdentity(a, b *FileIdentity) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.ContentHash != b.ContentHash {
		return false
	}
	if a.Size != b.Size {
		return false
	}
	// Compare ModTime with second precision
	if !a.ModTime.Truncate(time.Second).Equal(b.ModTime.Truncate(time.Second)) {
		return false
	}
	return true
}

// compareErrorDetails compares two ErrorDetails pointers for equality.
func compareErrorDetails(a, b *ErrorDetails) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.ErrorType == b.ErrorType &&
		a.ErrorMessage == b.ErrorMessage &&
		a.Operation == b.Operation
}

// compareMetadata compares two metadata maps for equality.
func compareMetadata(a, b map[string]string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

// Unit tests for event serialization
// Requirements: 3.3, 3.4

func TestEventSerialization_MoveEvent(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp:       ts,
		RunID:           "abc-123-def",
		EventType:       EventMove,
		Status:          StatusSuccess,
		SourcePath:      "/source/file.pdf",
		DestinationPath: "/dest/file.pdf",
		FileIdentity: &FileIdentity{
			ContentHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			Size:        1024,
			ModTime:     ts,
		},
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	if !contains(jsonStr, `"eventType":"MOVE"`) {
		t.Error("JSON should contain eventType MOVE")
	}
	if !contains(jsonStr, `"sourcePath":"/source/file.pdf"`) {
		t.Error("JSON should contain sourcePath")
	}
	if !contains(jsonStr, `"destinationPath":"/dest/file.pdf"`) {
		t.Error("JSON should contain destinationPath")
	}
	if !contains(jsonStr, `"fileIdentity"`) {
		t.Error("JSON should contain fileIdentity")
	}
}

func TestEventSerialization_SkipEvent(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp:  ts,
		RunID:      "abc-123-def",
		EventType:  EventSkip,
		Status:     StatusSkipped,
		SourcePath: "/source/file.pdf",
		ReasonCode: ReasonNoMatch,
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"eventType":"SKIP"`) {
		t.Error("JSON should contain eventType SKIP")
	}
	if !contains(jsonStr, `"reasonCode":"NO_MATCH"`) {
		t.Error("JSON should contain reasonCode")
	}
}

func TestEventSerialization_RouteToReviewEvent(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp:       ts,
		RunID:           "abc-123-def",
		EventType:       EventRouteToReview,
		Status:          StatusSuccess,
		SourcePath:      "/source/file.pdf",
		DestinationPath: "/review/file.pdf",
		ReasonCode:      ReasonUnclassified,
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"eventType":"ROUTE_TO_REVIEW"`) {
		t.Error("JSON should contain eventType ROUTE_TO_REVIEW")
	}
	if !contains(jsonStr, `"reasonCode":"UNCLASSIFIED"`) {
		t.Error("JSON should contain reasonCode UNCLASSIFIED")
	}
}

func TestEventSerialization_ErrorEvent(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp:  ts,
		RunID:      "abc-123-def",
		EventType:  EventError,
		Status:     StatusFailure,
		SourcePath: "/source/file.pdf",
		ErrorDetails: &ErrorDetails{
			ErrorType:    "IO_ERROR",
			ErrorMessage: "permission denied",
			Operation:    "move",
		},
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"eventType":"ERROR"`) {
		t.Error("JSON should contain eventType ERROR")
	}
	if !contains(jsonStr, `"errorDetails"`) {
		t.Error("JSON should contain errorDetails")
	}
	if !contains(jsonStr, `"errorType":"IO_ERROR"`) {
		t.Error("JSON should contain errorType")
	}
}

func TestEventSerialization_RunStartEvent(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp: ts,
		RunID:     "abc-123-def",
		EventType: EventRunStart,
		Status:    StatusSuccess,
		Metadata: map[string]string{
			"appVersion": "1.0.0",
			"machineId":  "machine-123",
		},
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, `"eventType":"RUN_START"`) {
		t.Error("JSON should contain eventType RUN_START")
	}
	if !contains(jsonStr, `"metadata"`) {
		t.Error("JSON should contain metadata")
	}
}

func TestEventSerialization_OptionalFieldsOmitted(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp: ts,
		RunID:     "abc-123-def",
		EventType: EventRunStart,
		Status:    StatusSuccess,
		// All optional fields are empty/nil
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)

	// Optional fields should be omitted when empty/nil
	if contains(jsonStr, `"sourcePath"`) {
		t.Error("JSON should not contain sourcePath when empty")
	}
	if contains(jsonStr, `"destinationPath"`) {
		t.Error("JSON should not contain destinationPath when empty")
	}
	if contains(jsonStr, `"reasonCode"`) {
		t.Error("JSON should not contain reasonCode when empty")
	}
	if contains(jsonStr, `"fileIdentity"`) {
		t.Error("JSON should not contain fileIdentity when nil")
	}
	if contains(jsonStr, `"errorDetails"`) {
		t.Error("JSON should not contain errorDetails when nil")
	}
	if contains(jsonStr, `"metadata"`) {
		t.Error("JSON should not contain metadata when nil")
	}
}

func TestEventSerialization_TimestampISO8601(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 45, 0, time.UTC)
	event := AuditEvent{
		Timestamp: ts,
		RunID:     "abc-123-def",
		EventType: EventRunStart,
		Status:    StatusSuccess,
	}

	data, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	jsonStr := string(data)
	// RFC3339 format: 2024-03-15T10:30:45Z
	if !contains(jsonStr, `"timestamp":"2024-03-15T10:30:45Z"`) {
		t.Errorf("Timestamp should be in ISO 8601 format, got: %s", jsonStr)
	}
}

func TestEventSerialization_AllEventTypes(t *testing.T) {
	eventTypes := []EventType{
		EventRunStart, EventRunEnd,
		EventMove, EventRouteToReview, EventSkip,
		EventDuplicateDetected, EventParseFailure, EventValidationFailure, EventError,
		EventUndoMove, EventUndoSkip, EventIdentityMismatch, EventAmbiguousIdentity,
		EventCollision, EventSourceMissing, EventContentChanged, EventConflictDetected,
		EventRotation, EventRetentionPrune, EventLogInitialized,
	}

	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	for _, et := range eventTypes {
		event := AuditEvent{
			Timestamp: ts,
			RunID:     "test-run",
			EventType: et,
			Status:    StatusSuccess,
		}

		data, err := event.MarshalJSON()
		if err != nil {
			t.Errorf("MarshalJSON failed for EventType %s: %v", et, err)
			continue
		}

		var restored AuditEvent
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Errorf("UnmarshalJSON failed for EventType %s: %v", et, err)
			continue
		}

		if restored.EventType != et {
			t.Errorf("EventType mismatch: expected %s, got %s", et, restored.EventType)
		}
	}
}

func TestUnmarshalJSONLine(t *testing.T) {
	jsonLine := `{"timestamp":"2024-03-15T10:30:00Z","runId":"abc-123","eventType":"MOVE","status":"SUCCESS","sourcePath":"/src/file.pdf","destinationPath":"/dst/file.pdf"}`

	event, err := UnmarshalJSONLine([]byte(jsonLine))
	if err != nil {
		t.Fatalf("UnmarshalJSONLine failed: %v", err)
	}

	if event.EventType != EventMove {
		t.Errorf("Expected EventMove, got %s", event.EventType)
	}
	if event.SourcePath != "/src/file.pdf" {
		t.Errorf("Expected /src/file.pdf, got %s", event.SourcePath)
	}
	if event.DestinationPath != "/dst/file.pdf" {
		t.Errorf("Expected /dst/file.pdf, got %s", event.DestinationPath)
	}
}

func TestMarshalJSONLine(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	event := AuditEvent{
		Timestamp: ts,
		RunID:     "abc-123",
		EventType: EventMove,
		Status:    StatusSuccess,
	}

	data, err := event.MarshalJSONLine()
	if err != nil {
		t.Fatalf("MarshalJSONLine failed: %v", err)
	}

	// Should not contain newline
	if contains(string(data), "\n") {
		t.Error("MarshalJSONLine should not contain newline")
	}

	// Should be valid JSON
	var restored AuditEvent
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Errorf("Output should be valid JSON: %v", err)
	}
}

// contains checks if substr is in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
