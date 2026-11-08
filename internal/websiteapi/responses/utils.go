package responses

import (
	"encoding/json"
	"net/http"
	"ttnmapper-services/pkg/database"

	"github.com/go-chi/render"
)

func RenderError(writer http.ResponseWriter, request *http.Request, err error) {
	errorResponse := ErrorResponse{}
	errorResponse.Success = false
	errorResponse.Message = err.Error()
	render.JSON(writer, request, errorResponse)
}

func DbGatewaysToResponse(dbGateways []database.Gateway) []Gateway {
	responseGateways := make([]Gateway, 0)

	for _, gateway := range dbGateways {
		responseGw := DbGatewayToResponse(gateway)
		responseGateways = append(responseGateways, responseGw)
	}

	return responseGateways
}

func DbGatewayToResponse(gateway database.Gateway) Gateway {

	responseGw := Gateway{
		DatabaseId: gateway.ID,
		NetworkId:  gateway.NetworkId,
		GatewayId:  gateway.GatewayId,
		LastHeard:  gateway.LastHeard,
		Latitude:   gateway.Latitude,
		Longitude:  gateway.Longitude,
		Altitude:   gateway.Altitude,
	}

	if gateway.GatewayEui != nil {
		responseGw.GatewayEUI = *gateway.GatewayEui
	}
	if gateway.Name != nil {
		responseGw.Name = *gateway.Name
	}

	attributes := make(map[string]interface{}, 0)
	err := json.Unmarshal(gateway.Attributes, &attributes)
	if err != nil {
		// nothing
	}
	responseGw.Attributes = attributes

	return responseGw
}

func DbGatewaysWithBoundingBoxToResponse(dbGateways []database.GatewayWithBoundingBox) []Gateway {
	responseGateways := make([]Gateway, 0)

	for _, gateway := range dbGateways {
		responseGw := DbGatewayWithBoundingBoxToResponse(gateway)
		responseGateways = append(responseGateways, responseGw)
	}

	return responseGateways
}

func DbGatewayWithBoundingBoxToResponse(gateway database.GatewayWithBoundingBox) Gateway {

	responseGw := Gateway{
		DatabaseId: gateway.Gateway.ID,
		NetworkId:  gateway.Gateway.NetworkId,
		GatewayId:  gateway.Gateway.GatewayId,
		LastHeard:  gateway.LastHeard,
		Latitude:   gateway.Latitude,
		Longitude:  gateway.Longitude,
		Altitude:   gateway.Altitude,
		North:      gateway.North,
		South:      gateway.South,
		West:       gateway.West,
		East:       gateway.East,
	}

	if gateway.GatewayEui != nil {
		responseGw.GatewayEUI = *gateway.GatewayEui
	}
	if gateway.Name != nil {
		responseGw.Name = *gateway.Name
	}

	attributes := make(map[string]interface{}, 0)
	err := json.Unmarshal(gateway.Attributes, &attributes)
	if err != nil {
		// nothing
	}
	responseGw.Attributes = attributes

	return responseGw
}

func AnonymiseDeviceMeasurement(inputData []DeviceMeasurement) []DeviceMeasurement {
	result := make([]DeviceMeasurement, 0)

	for _, measurement := range inputData {
		outputMeasurement := measurement

		// Device IDs
		outputMeasurement.DevId = "<redacted>"
		outputMeasurement.DevEui = "<redacted>"
		outputMeasurement.AppId = "<redacted>"

		// Port, count
		outputMeasurement.FPort = 1
		outputMeasurement.FCnt = 1

		// Time
		outputMeasurement.Time = TruncateToDay(outputMeasurement.Time)
		outputMeasurement.GatewayTime = TruncateToDay(outputMeasurement.GatewayTime)
		outputMeasurement.FineTimestamp = 0

		outputMeasurement.UserAgent = "<redacted>"

		result = append(result, outputMeasurement)
	}

	return result
}
