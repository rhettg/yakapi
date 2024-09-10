package sfc

import (
	"encoding/json"
	"fmt"
)

type RegionMap map[Region]interface{}

func NewRegionMap() RegionMap {
	return make(RegionMap)
}

func (rm RegionMap) SetFloat(region Region, value float64) {
	rm[region] = value
}

func (rm RegionMap) SetString(region Region, value string) {
	rm[region] = value
}

func (rm RegionMap) Get(region Region) (interface{}, bool) {
	value, ok := rm[region]
	return value, ok
}

func (rm RegionMap) ToJSON() (string, error) {
	jsonData, err := json.Marshal(rm)
	if err != nil {
		return "", fmt.Errorf("error marshaling RegionMap to JSON: %w", err)
	}
	return string(jsonData), nil
}

func (rm RegionMap) FromJSON(jsonStr string) error {
	return json.Unmarshal([]byte(jsonStr), &rm)
}
