// SPDX-License-Identifier: Apache-2.0
package proto

var profileByName = map[string]int{
	"ulbw": ProfileBandwidthUltraLow,
	"vlbw": ProfileBandwidthVeryLow,
	"lbw":  ProfileBandwidthLow,
	"mq":   ProfileQualityMedium,
	"hq":   ProfileQualityHigh,
	"shq":  ProfileQualityMax,
	"ll":   ProfileLatencyLow,
	"ull":  ProfileLatencyUltraLow,
}

func ProfileFromName(name string) int {
	if p, ok := profileByName[name]; ok {
		return p
	}
	return DefaultProfile
}

var modeByName = map[string]int{
	"full": ModeFullDuplex,
	"fdx":  ModeFullDuplex,
	"half": ModeHalfDuplex,
	"hdx":  ModeHalfDuplex,
	"ptt":  ModeHalfDuplex,
}

func ModeFromName(name string) int {
	if m, ok := modeByName[name]; ok {
		return m
	}
	return DefaultMode
}

func ModeName(mode int) string {
	switch mode {
	case ModeFullDuplex:
		return "full"
	case ModeHalfDuplex:
		return "half"
	default:
		return "unknown"
	}
}
