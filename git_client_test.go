package main

import (
	"testing"
)

func TestObfuscateSecretData(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Obfuscate data field",
			input: map[string]interface{}{
				"data": map[string]interface{}{
					"username": "YWRtaW4=",
					"password": "cGFzc3dvcmQ=",
				},
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{
					"username": "********",
					"password": "********",
				},
			},
		},
		{
			name: "Obfuscate stringData field",
			input: map[string]interface{}{
				"stringData": map[string]interface{}{
					"api-key": "sk-1234567890",
					"token":   "ghp_abc123",
				},
			},
			expected: map[string]interface{}{
				"stringData": map[string]interface{}{
					"api-key": "********",
					"token":   "********",
				},
			},
		},
		{
			name: "Obfuscate both data and stringData",
			input: map[string]interface{}{
				"data": map[string]interface{}{
					"username": "YWRtaW4=",
				},
				"stringData": map[string]interface{}{
					"password": "secret123",
				},
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{
					"username": "********",
				},
				"stringData": map[string]interface{}{
					"password": "********",
				},
			},
		},
		{
			name: "Empty secret",
			input: map[string]interface{}{
				"data": map[string]interface{}{},
			},
			expected: map[string]interface{}{
				"data": map[string]interface{}{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obfuscateSecretData(tt.input)

			// Check data field
			if data, ok := tt.input["data"].(map[string]interface{}); ok {
				expectedData := tt.expected["data"].(map[string]interface{})
				for key := range expectedData {
					if data[key] != "********" {
						t.Errorf("Expected data[%s] to be ********, got %v", key, data[key])
					}
				}
			}

			// Check stringData field
			if stringData, ok := tt.input["stringData"].(map[string]interface{}); ok {
				expectedStringData := tt.expected["stringData"].(map[string]interface{})
				for key := range expectedStringData {
					if stringData[key] != "********" {
						t.Errorf("Expected stringData[%s] to be ********, got %v", key, stringData[key])
					}
				}
			}
		})
	}
}

func TestDetectSecretChanges(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]interface{}
		new      map[string]interface{}
		expected bool
	}{
		{
			name: "No changes",
			existing: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
					"password": "********",
				},
				"metadata": map[string]interface{}{
					"name": "test-secret",
				},
			},
			new: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
					"password": "********",
				},
				"metadata": map[string]interface{}{
					"name": "test-secret",
				},
			},
			expected: false,
		},
		{
			name: "Key added",
			existing: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
				},
			},
			new: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
					"password": "********",
				},
			},
			expected: true,
		},
		{
			name: "Key removed",
			existing: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
					"password": "********",
				},
			},
			new: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
				},
			},
			expected: true,
		},
		{
			name: "Type changed",
			existing: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
				},
			},
			new: map[string]interface{}{
				"type": "kubernetes.io/tls",
				"data": map[string]interface{}{
					"username": "********",
				},
			},
			expected: true,
		},
		{
			name: "Label added",
			existing: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
				},
				"metadata": map[string]interface{}{
					"name": "test-secret",
				},
			},
			new: map[string]interface{}{
				"type": "Opaque",
				"data": map[string]interface{}{
					"username": "********",
				},
				"metadata": map[string]interface{}{
					"name": "test-secret",
					"labels": map[string]interface{}{
						"env": "production",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectSecretChanges(tt.existing, tt.new)
			if result != tt.expected {
				t.Errorf("Expected detectSecretChanges to return %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "Equal slices",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "Equal slices (different order)",
			a:        []string{"a", "b", "c"},
			b:        []string{"c", "b", "a"},
			expected: true,
		},
		{
			name:     "Different lengths",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "Different elements",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "Empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSlicesEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Expected stringSlicesEqual to return %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]interface{}
		b        map[string]interface{}
		expected bool
	}{
		{
			name: "Equal maps",
			a: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			b: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: true,
		},
		{
			name: "Different values",
			a: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			b: map[string]interface{}{
				"key1": "value1",
				"key2": "different",
			},
			expected: false,
		},
		{
			name: "Different keys",
			a: map[string]interface{}{
				"key1": "value1",
			},
			b: map[string]interface{}{
				"key2": "value2",
			},
			expected: false,
		},
		{
			name:     "Empty maps",
			a:        map[string]interface{}{},
			b:        map[string]interface{}{},
			expected: true,
		},
		{
			name: "Different lengths",
			a: map[string]interface{}{
				"key1": "value1",
			},
			b: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Expected mapsEqual to return %v, got %v", tt.expected, result)
			}
		})
	}
}
