// Package types retains local names for service-specific models while message
// contracts are defined once in the root package.
package types

import shared "ttnmapper-services/pkg/types"

type TtnMapperUplinkMessage = shared.TtnMapperUplinkMessage
type TtnMapperGateway = shared.TtnMapperGateway
type TtnMapperGatewayMoved = shared.TtnMapperGatewayMoved

const (
	NS_TTN_V2   = shared.NS_TTN_V2
	NS_TTS_V3   = shared.NS_TTS_V3
	NS_CHIRP    = shared.NS_CHIRP
	NS_HELIUM   = shared.NS_HELIUM
	NS_THINGSIX = shared.NS_THINGSIX
	NS_UNKNOWN  = shared.NS_UNKNOWN
	MOD_LORA    = shared.MOD_LORA
	MOD_FSK     = shared.MOD_FSK
	MOD_LRFHSS  = shared.MOD_LRFHSS
)
