package thethingsstack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go.thethings.network/lorawan-stack/v3/pkg/jsonpb"
	"go.thethings.network/lorawan-stack/v3/pkg/ttnpb"
	"io"
	"log"
	"net/http"
	"time"
	"ttnmapper-services/pkg/types"
)

func FetchGateways(tenantId string, apiKey string) ([]Gateway, error) {
	log.Printf("Fetching TTS Api Gateways for tenant %s", tenantId)

	url := fmt.Sprintf("https://%s.eu1.cloud.thethings.industries/api/v3/gateways?"+
		"field_mask=ids,created_at,updated_at,deleted_at,name,description,attributes,contact_info,version_ids,"+
		"gateway_server_address,auto_update,frequency_plan_id,frequency_plan_ids,antennas,status_public,"+
		"location_public,schedule_downlink_late,enforce_duty_cycle,downlink_path_constraint,schedule_anytime_delay,"+
		"update_location_from_status,lbs_lns_secret,claim_authentication_code,target_cups_uri,target_cups_key,"+
		"require_authenticated_connection,lrfhss,disable_packet_broker_forwarding", tenantId)

	log.Println(url)

	httpClient := http.Client{
		Timeout: time.Second * 60, // Maximum of 1 minute
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ttnmapper-update-gateway")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	webData := V3Gateways{}
	err = json.NewDecoder(res.Body).Decode(&webData)
	if err != nil {
		return nil, err
	}
	return webData.Gateways, nil
}

func FetchStatusesBatch(gateways []Gateway, apiKey string) (ttnpb.BatchGetGatewayConnectionStatsResponse, error) {
	// Use gateway_server_address with endpoint
	// /api/v3/gs/gateways/connection/stats

	url := fmt.Sprintf("https://%s/api/v3/gs/gateways/connection/stats", gateways[0].GatewayServerAddress)
	httpClient := http.Client{
		Timeout: time.Second * 60, // Maximum of 1 minute
	}

	log.Println("URL", url)

	postData := GatewayStatsBatchRequest{}
	for _, gateway := range gateways {
		postData.GatewayIds = append(postData.GatewayIds, gateway.Ids)
		//postData.GatewayIds = append(postData.GatewayIds, GatewayIds{GatewayId: gateway.Ids.GatewayId})
	}
	postJson, err := json.Marshal(postData)

	log.Println(string(postJson))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(postJson))
	if err != nil {
		return ttnpb.BatchGetGatewayConnectionStatsResponse{}, err
	}

	req.Header.Set("User-Agent", "ttnmapper-update-gateway")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := httpClient.Do(req)
	if err != nil {
		return ttnpb.BatchGetGatewayConnectionStatsResponse{}, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ttnpb.BatchGetGatewayConnectionStatsResponse{}, err
	}

	webData := ttnpb.BatchGetGatewayConnectionStatsResponse{}
	marshaler := jsonpb.TTN()
	err = marshaler.Unmarshal(body, &webData)
	if err != nil {
		return ttnpb.BatchGetGatewayConnectionStatsResponse{}, err
	}
	return webData, nil
}

func FetchStatus(gateway Gateway, apiKey string) (Status, error) {
	// Use gateway_server_address with endpoint
	// /api/v3/gs/gateways/{gateway_id}/connection/stats
	log.Printf("Getting status for gateway %s", gateway.Ids.GatewayId)

	url := fmt.Sprintf("https://%s/api/v3/gs/gateways/%s/connection/stats", gateway.GatewayServerAddress, gateway.Ids.GatewayId)

	httpClient := http.Client{
		Timeout: time.Second * 60, // Maximum of 1 minute
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Status{}, err
	}

	req.Header.Set("User-Agent", "ttnmapper-update-gateway")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := httpClient.Do(req)
	if err != nil {
		return Status{}, err
	}

	defer res.Body.Close()

	webData := Status{}
	err = json.NewDecoder(res.Body).Decode(&webData)
	if err != nil {
		return Status{}, err
	}
	return webData, nil

}

func TtsApiGatewayToTtnMapperGateway(tenantId string, gatewayIn Gateway, statusIn *ttnpb.GatewayConnectionStats) (types.TtnMapperGateway, error) {

	if statusIn == nil {
		return types.TtnMapperGateway{}, errors.New("")
	}

	lastHeard := time.Time{}

	if statusIn.LastStatusReceivedAt != nil {
		lastStatus := time.Unix(statusIn.LastStatusReceivedAt.Seconds, int64(statusIn.LastStatusReceivedAt.Nanos))
		if lastStatus.After(lastHeard) {
			lastHeard = lastStatus
		}
	}
	if statusIn.LastUplinkReceivedAt != nil {
		lastUplink := time.Unix(statusIn.LastUplinkReceivedAt.Seconds, int64(statusIn.LastUplinkReceivedAt.Nanos))
		if lastUplink.After(lastHeard) {
			lastHeard = lastUplink
		}
	}

	latitude := 0.0
	longitude := 0.0
	var altitude int32
	var accuracy int32
	locationSource := ""

	// We don't need perfectly accurate location, so we just use the location of the first antenna
	if len(gatewayIn.Antennas) > 0 {
		latitude = gatewayIn.Antennas[0].Location.Latitude
		longitude = gatewayIn.Antennas[0].Location.Longitude
		altitude = gatewayIn.Antennas[0].Location.Altitude
		accuracy = gatewayIn.Antennas[0].Location.Accuracy
		locationSource = gatewayIn.Antennas[0].Location.Source
	}

	var gatewayOut types.TtnMapperGateway

	gatewayOut = types.TtnMapperGateway{
		NetworkId:                   "NS_TTS_V3://" + tenantId + "@000013",
		GatewayId:                   gatewayIn.Ids.GatewayId,
		GatewayEui:                  gatewayIn.Ids.Eui,
		AntennaIndex:                0,
		Time:                        lastHeard.UnixNano(),
		Timestamp:                   0,
		FineTimestamp:               0,
		FineTimestampEncrypted:      nil,
		FineTimestampEncryptedKeyId: "",
		ChannelIndex:                0,
		Rssi:                        0,
		SignalRssi:                  0,
		Snr:                         0,
		Latitude:                    latitude,
		Longitude:                   longitude,
		Altitude:                    altitude,
		LocationAccuracy:            accuracy,
		LocationSource:              locationSource,
		Name:                        gatewayIn.Name,
	}
	gatewayOut.Attributes = make(map[string]interface{}, 0)
	gatewayOut.Attributes["description"] = gatewayIn.Description
	gatewayOut.Attributes["cluster_id"] = gatewayIn.GatewayServerAddress
	gatewayOut.Attributes["frequency_plans"] = gatewayIn.FrequencyPlanIds

	return gatewayOut, nil
}
