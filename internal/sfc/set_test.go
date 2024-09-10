package sfc

import "testing"

func TestSemicolonSeparatedString_SetStrOf(t *testing.T) {
	testCases := []struct {
		name     string
		initial  string
		region   Region
		value    string
		expected string
	}{
		{
			name:     "Replace middle element",
			initial:  "part1;part2;part3",
			region:   RegionB,
			value:    "newpart2",
			expected: "part1;newpart2;part3",
		},
		{
			name:     "Replace first element",
			initial:  "part1;part2;part3",
			region:   RegionA,
			value:    "newpart1",
			expected: "newpart1;part2;part3",
		},
		{
			name:     "Append new element",
			initial:  "part1;part2;part3",
			region:   RegionD,
			value:    "part4",
			expected: "part1;part2;part3;part4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			str := SemicolonSeparatedString(tc.initial)
			str.SetStrOf(tc.region, tc.value)
			if string(str) != tc.expected {
				t.Errorf("Expected %q, but got %q", tc.expected, str)
			}
		})
	}
}
