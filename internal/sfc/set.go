package sfc

import "strings"

type Region uint8

const (
	RegionA Region = 0
	RegionB Region = 1
	RegionC Region = 2
	RegionD Region = 3
	RegionE Region = 4
	RegionF Region = 5
	RegionG Region = 6
	RegionH Region = 7
	RegionI Region = 8
	RegionJ Region = 9
	RegionK Region = 10
	RegionL Region = 11
	RegionM Region = 12
	RegionN Region = 13
	RegionO Region = 14
	RegionP Region = 15
	RegionQ Region = 16
	RegionR Region = 17
	RegionS Region = 18
	RegionT Region = 19
	RegionU Region = 20
	RegionV Region = 21
	RegionW Region = 22
	RegionX Region = 23
	RegionY Region = 24
	RegionZ Region = 25
)

type SemicolonSeparatedString string

func (s *SemicolonSeparatedString) SetStrOf(region Region, value string) {
	str := string(*s)
	parts := strings.Split(str, ";")

	if int(region) < len(parts) {
		parts[region] = value
	} else if int(region) == len(parts) {
		parts = append(parts, value)
	}

	*s = SemicolonSeparatedString(strings.Join(parts, ";"))
}
