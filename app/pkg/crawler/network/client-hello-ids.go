package network

import (
	utls "github.com/refraction-networking/utls"
)

type ClientHelloID = utls.ClientHelloID

var clientHelloIDs = map[string]map[string]*ClientHelloID{
	"Firefox": {
		"99":  &utls.HelloFirefox_99,
		"102": &utls.HelloFirefox_102,
		"105": &utls.HelloFirefox_105,
		"120": &utls.HelloFirefox_120,
	},
	"Chrome": {
		"83":  &utls.HelloChrome_83,
		"87":  &utls.HelloChrome_87,
		"96":  &utls.HelloChrome_96,
		"102": &utls.HelloChrome_102,
		"106": &utls.HelloChrome_106_Shuffle,
		"120": &utls.HelloChrome_120,
	},
	"Edge": {
		"85":  &utls.HelloEdge_85,
		"106": &utls.HelloEdge_106,
	},
	"Safari": {
		"16": &utls.HelloSafari_16_0,
	},
	"iOS": { // currently not supported
		"11": &utls.HelloIOS_11_1,
		"12": &utls.HelloIOS_12_1,
		"13": &utls.HelloIOS_13,
		"14": &utls.HelloIOS_14,
	},
}
