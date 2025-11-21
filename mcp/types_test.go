package mcp

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaMarshalling(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		setupMeta  func() *Meta
		expToken   ProgressToken
		expFields  map[string]any
	}{
		{
			name:      "empty",
			json:      "{}",
			setupMeta: func() *Meta { return &Meta{} },
			expToken:  nil,
			expFields: map[string]any{}, // Unmarshaling creates empty map, not nil
		},
		{
			name:      "empty additional fields",
			json:      "{}",
			setupMeta: func() *Meta { m := &Meta{}; m.SetAdditionalFields(map[string]any{}); return m },
			expToken:  nil,
			expFields: map[string]any{},
		},
		{
			name:      "string token only",
			json:      `{"progressToken":"123"}`,
			setupMeta: func() *Meta { return &Meta{ProgressToken: "123"} },
			expToken:  "123",
			expFields: map[string]any{}, // Unmarshaling creates empty map, not nil
		},
		{
			name:      "string token only, empty additional fields",
			json:      `{"progressToken":"123"}`,
			setupMeta: func() *Meta { m := &Meta{ProgressToken: "123"}; m.SetAdditionalFields(map[string]any{}); return m },
			expToken:  "123",
			expFields: map[string]any{},
		},
		{
			name:      "additional fields only",
			json:      `{"a":2,"b":"1"}`,
			setupMeta: func() *Meta { m := &Meta{}; m.SetAdditionalFields(map[string]any{"a": 2, "b": "1"}); return m },
			expToken:  nil,
			// For untyped map, numbers are always float64
			expFields: map[string]any{"a": float64(2), "b": "1"},
		},
		{
			name:      "progress token and additional fields",
			json:      `{"a":2,"b":"1","progressToken":"123"}`,
			setupMeta: func() *Meta { m := &Meta{ProgressToken: "123"}; m.SetAdditionalFields(map[string]any{"a": 2, "b": "1"}); return m },
			expToken:  "123",
			// For untyped map, numbers are always float64
			expFields: map[string]any{"a": float64(2), "b": "1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := tc.setupMeta()

			data, err := json.Marshal(meta)
			require.NoError(t, err)
			assert.Equal(t, tc.json, string(data))

			unmarshaled := &Meta{}
			err = json.Unmarshal([]byte(tc.json), unmarshaled)
			require.NoError(t, err)

			// Compare fields individually since we can't directly compare structs with unexported fields
			assert.Equal(t, tc.expToken, unmarshaled.ProgressToken)
			assert.Equal(t, tc.expFields, unmarshaled.GetAdditionalFields())
		})
	}
}

func TestResourceLinkSerialization(t *testing.T) {
	resourceLink := NewResourceLink(
		"file:///example/document.pdf",
		"Sample Document",
		"A sample document for testing",
		"application/pdf",
	)

	// Test marshaling
	data, err := json.Marshal(resourceLink)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled ResourceLink
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, "resource_link", unmarshaled.Type)
	assert.Equal(t, "file:///example/document.pdf", unmarshaled.URI)
	assert.Equal(t, "Sample Document", unmarshaled.Name)
	assert.Equal(t, "A sample document for testing", unmarshaled.Description)
	assert.Equal(t, "application/pdf", unmarshaled.MIMEType)
}

func TestCallToolResultWithResourceLink(t *testing.T) {
	result := &CallToolResult{
		Content: []Content{
			TextContent{
				Type: "text",
				Text: "Here's a resource link:",
			},
			NewResourceLink(
				"file:///example/test.pdf",
				"Test Document",
				"A test document",
				"application/pdf",
			),
		},
		IsError: false,
	}

	// Test marshaling
	data, err := json.Marshal(result)
	require.NoError(t, err)

	// Test unmarshalling
	var unmarshalled CallToolResult
	err = json.Unmarshal(data, &unmarshalled)
	require.NoError(t, err)

	// Verify content
	require.Len(t, unmarshalled.Content, 2)

	// Check first content (TextContent)
	textContent, ok := unmarshalled.Content[0].(TextContent)
	require.True(t, ok)
	assert.Equal(t, "text", textContent.Type)
	assert.Equal(t, "Here's a resource link:", textContent.Text)

	// Check second content (ResourceLink)
	resourceLink, ok := unmarshalled.Content[1].(ResourceLink)
	require.True(t, ok)
	assert.Equal(t, "resource_link", resourceLink.Type)
	assert.Equal(t, "file:///example/test.pdf", resourceLink.URI)
	assert.Equal(t, "Test Document", resourceLink.Name)
	assert.Equal(t, "A test document", resourceLink.Description)
	assert.Equal(t, "application/pdf", resourceLink.MIMEType)
}

func TestResourceContentsMetaField(t *testing.T) {
	tests := []struct {
		name         string
		inputJSON    string
		expectedType string
		expectedMeta map[string]any
	}{
		{
			name: "TextResourceContents with empty _meta",
			inputJSON: `{
				"uri":"file://empty-meta.txt",
				"mimeType":"text/plain",
				"text":"x",
				"_meta": {}
			}`,
			expectedType: "text",
			expectedMeta: map[string]any{},
		},
		{
			name: "TextResourceContents with _meta field",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": {
					"mcpui.dev/ui-preferred-frame-size": ["800px", "600px"],
					"mcpui.dev/ui-initial-render-data": {
						"test": "value"
					}
				}
			}`,
			expectedType: "text",
			expectedMeta: map[string]any{
				"mcpui.dev/ui-preferred-frame-size": []interface{}{"800px", "600px"},
				"mcpui.dev/ui-initial-render-data": map[string]any{
					"test": "value",
				},
			},
		},
		{
			name: "BlobResourceContents with _meta field",
			inputJSON: `{
				"uri": "file://image.png",
				"mimeType": "image/png",
				"blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
				"_meta": {
					"width": 100,
					"height": 100,
					"format": "PNG"
				}
			}`,
			expectedType: "blob",
			expectedMeta: map[string]any{
				"width":  float64(100), // JSON numbers are always float64
				"height": float64(100),
				"format": "PNG",
			},
		},
		{
			name: "TextResourceContents without _meta field",
			inputJSON: `{
				"uri": "file://simple.txt",
				"mimeType": "text/plain",
				"text": "Simple content"
			}`,
			expectedType: "text",
			expectedMeta: nil,
		},
		{
			name: "BlobResourceContents without _meta field",
			inputJSON: `{
				"uri": "file://simple.png",
				"mimeType": "image/png",
				"blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="
			}`,
			expectedType: "blob",
			expectedMeta: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the JSON as a generic map first
			var contentMap map[string]any
			err := json.Unmarshal([]byte(tc.inputJSON), &contentMap)
			require.NoError(t, err)

			// Use ParseResourceContents to convert to ResourceContents
			resourceContent, err := ParseResourceContents(contentMap)
			require.NoError(t, err)
			require.NotNil(t, resourceContent)

			switch tc.expectedType {
			case "text":
				textContent, ok := resourceContent.(TextResourceContents)
				require.True(t, ok, "Expected TextResourceContents")

				assert.Equal(t, contentMap["uri"], textContent.URI)
				assert.Equal(t, contentMap["mimeType"], textContent.MIMEType)
				assert.Equal(t, contentMap["text"], textContent.Text)

				assert.Equal(t, tc.expectedMeta, textContent.Meta)

			case "blob":
				blobContent, ok := resourceContent.(BlobResourceContents)
				require.True(t, ok, "Expected BlobResourceContents")

				assert.Equal(t, contentMap["uri"], blobContent.URI)
				assert.Equal(t, contentMap["mimeType"], blobContent.MIMEType)
				assert.Equal(t, contentMap["blob"], blobContent.Blob)

				assert.Equal(t, tc.expectedMeta, blobContent.Meta)
			}

			// Test round-trip marshaling to ensure _meta is preserved
			marshaledJSON, err := json.Marshal(resourceContent)
			require.NoError(t, err)

			var marshaledMap map[string]any
			err = json.Unmarshal(marshaledJSON, &marshaledMap)
			require.NoError(t, err)

			// Verify _meta field is preserved in marshaled output
			v, ok := marshaledMap["_meta"]
			if tc.expectedMeta != nil {
				// Special case: empty maps are omitted due to omitempty tag
				if len(tc.expectedMeta) == 0 {
					assert.False(t, ok, "_meta should be omitted when empty due to omitempty")
				} else {
					require.True(t, ok, "_meta should be present")
					assert.Equal(t, tc.expectedMeta, v)
				}
			} else {
				assert.False(t, ok, "_meta should be omitted when nil")
			}
		})
	}
}

func TestParseResourceContentsInvalidMeta(t *testing.T) {
	tests := []struct {
		name        string
		inputJSON   string
		expectedErr string
	}{
		{
			name: "TextResourceContents with invalid _meta (string)",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": "invalid_meta_string"
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "TextResourceContents with invalid _meta (number)",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": 123
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "TextResourceContents with invalid _meta (array)",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": ["invalid", "array"]
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "TextResourceContents with invalid _meta (boolean)",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": true
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "TextResourceContents with invalid _meta (null)",
			inputJSON: `{
				"uri": "file://test.txt",
				"mimeType": "text/plain",
				"text": "Hello World",
				"_meta": null
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "BlobResourceContents with invalid _meta (string)",
			inputJSON: `{
				"uri": "file://image.png",
				"mimeType": "image/png",
				"blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
				"_meta": "invalid_meta_string"
			}`,
			expectedErr: "_meta must be an object",
		},
		{
			name: "BlobResourceContents with invalid _meta (number)",
			inputJSON: `{
				"uri": "file://image.png",
				"mimeType": "image/png",
				"blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
				"_meta": 456
			}`,
			expectedErr: "_meta must be an object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the JSON as a generic map first
			var contentMap map[string]any
			err := json.Unmarshal([]byte(tc.inputJSON), &contentMap)
			require.NoError(t, err)

			// Use ParseResourceContents to convert to ResourceContents
			resourceContent, err := ParseResourceContents(contentMap)

			// Expect an error
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
			assert.Nil(t, resourceContent)
		})
	}
}

func TestMetaConcurrentAccess(t *testing.T) {
	meta := &Meta{}
	meta.SetAdditionalFields(make(map[string]any))

	// Test concurrent writes and reads
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes using SetAdditionalField
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				meta.SetAdditionalField("key", j)
			}
		}(i)
	}

	// Concurrent marshaling (reads)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, err := json.Marshal(meta)
				require.NoError(t, err)
			}
		}()
	}

	// Concurrent reads using GetAdditionalFields
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = meta.GetAdditionalFields()
			}
		}()
	}

	wg.Wait()
	// If we get here without a panic, the concurrent access is safe
}
