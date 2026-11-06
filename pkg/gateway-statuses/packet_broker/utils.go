package packet_broker

import (
	"fmt"
	packetbroker "go.packetbroker.org/api/v3"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func RoutingPolicyToDbPolicy(in *packetbroker.RoutingPolicy, out *database.PacketBrokerRoutingPolicy) {
	//out.ID
	out.HomeNetworkId = TenantNetIdToNetworkID(in.HomeNetworkNetId, in.HomeNetworkTenantId)
	out.ForwarderNetworkId = TenantNetIdToNetworkID(in.ForwarderNetId, in.ForwarderTenantId)
	out.UplinkJoinRequest = in.Uplink.JoinRequest
	out.UplinkMacData = in.Uplink.MacData
	out.UplinkApplicationData = in.Uplink.ApplicationData
	out.UplinkSignalQuality = in.Uplink.SignalQuality
	out.UplinkLocalization = in.Uplink.Localization
	out.DownlinkJoinAccept = in.Downlink.JoinAccept
	out.DownlinkMacData = in.Downlink.MacData
	out.DownlinkApplicationData = in.Downlink.ApplicationData
}

func TenantNetIdToNetworkID(netId uint32, tenantId string) string {
	// TenantID is a TTS concept, so assume it's a TTS network if the tenant ID is not empty. TODO: verify ChirpStack
	if tenantId != "" {
		return fmt.Sprintf("%s://%s@%06X", types.NS_TTS_V3, tenantId, netId)
	}
	return fmt.Sprintf("%s://%06X", types.NS_UNKNOWN, netId)
}
