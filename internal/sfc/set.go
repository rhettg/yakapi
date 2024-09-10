package sfc

import (
	"encoding/json"
	"fmt"
)

type Region string

const (
	RegionA Region = "A"
	RegionB Region = "B"
	RegionC Region = "C"
	RegionD Region = "D"
	RegionE Region = "E"
	RegionF Region = "F"
	RegionG Region = "G"
	RegionH Region = "H"
	RegionI Region = "I"
	RegionJ Region = "J"
	RegionK Region = "K"
	RegionL Region = "L"
	RegionM Region = "M"
	RegionN Region = "N"
	RegionO Region = "O"
	RegionP Region = "P"
	RegionQ Region = "Q"
	RegionR Region = "R"
	RegionS Region = "S"
	RegionT Region = "T"
	RegionU Region = "U"
	RegionV Region = "V"
	RegionW Region = "W"
	RegionX Region = "X"
	RegionY Region = "Y"
	RegionZ Region = "Z"
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
