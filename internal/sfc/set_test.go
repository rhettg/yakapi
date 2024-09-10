package sfc

import (
	"encoding/json"
	"testing"
)

func TestRegionMap(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func(RegionMap)
		expected string
	}{
		{
			name: "Set float value",
			setup: func(rm RegionMap) {
				rm.SetFloat(RegionA, 50.0)
			},
			expected: `{"A":50}`,
		},
		{
			name: "Set string value",
			setup: func(rm RegionMap) {
				rm.SetString(RegionB, "test")
			},
			expected: `{"B":"test"}`,
		},
		{
			name: "Set multiple values",
			setup: func(rm RegionMap) {
				rm.SetFloat(RegionA, 50.0)
				rm.SetString(RegionB, "test")
				rm.SetFloat(RegionC, 75.5)
			},
			expected: `{"A":50,"B":"test","C":75.5}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rm := NewRegionMap()
			tc.setup(rm)

			jsonStr, err := rm.ToJSON()
			if err != nil {
				t.Fatalf("Failed to convert to JSON: %v", err)
			}

			var result map[string]interface{}
			var expected map[string]interface{}

			json.Unmarshal([]byte(jsonStr), &result)
			json.Unmarshal([]byte(tc.expected), &expected)

			if !mapsEqual(result, expected) {
				t.Errorf("Expected %s, but got %s", tc.expected, jsonStr)
			}
		})
	}
}

func mapsEqual(m1, m2 map[string]interface{}) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || v1 != v2 {
			return false
		}
	}
	return true
}
