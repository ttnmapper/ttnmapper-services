package gateway

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"ttnmapper-services/cmd/website-api/responses"
	"ttnmapper-services/pkg/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func Routes() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/{network_id}/{gateway_id}/details", GetGatewayDetails)
	router.Get("/data", GetGatewayData)
	router.Get("/{network_id}/{gateway_id}/radar/multi", GetGatewayRadarMulti)
	router.Get("/{network_id}/{gateway_id}/radar/single", GetGatewayRadarSingle)

	return router
}

func GetGatewayDetails(writer http.ResponseWriter, request *http.Request) {
	var err error

	networkId := chi.URLParam(request, "network_id")
	networkId, err = url.PathUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	if networkId == "" {
		responses.RenderError(writer, request, errors.New("network_id not set"))
		return
	}

	gatewayId := chi.URLParam(request, "gateway_id")
	gatewayId, err = url.PathUnescape(gatewayId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	if gatewayId == "" {
		responses.RenderError(writer, request, errors.New("gateway_id not set"))
		return
	}

	indexer := database.GatewayIndexer{NetworkId: networkId, GatewayId: gatewayId}
	gateway, err := database.GetGateway(indexer)
	if err != nil {
		log.Println(err.Error())
		//responses.RenderError(writer, request, err)
		//return
		gateway = database.Gateway{GatewayId: gatewayId, NetworkId: networkId}
	}

	responseGateway := responses.DbGatewayToResponse(gateway)
	render.JSON(writer, request, responseGateway)
}

func GetGatewayData(writer http.ResponseWriter, request *http.Request) {
	var err error

	gatewayId := request.URL.Query().Get("gateway_id")
	if gatewayId == "" {
		responses.RenderError(writer, request, errors.New("gateway_id not set"))
		return
	}
	gatewayId, err = url.QueryUnescape(gatewayId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	gatewayId = strings.Trim(gatewayId, " ") // ignore spaces before and after
	//gatewayId = strings.Replace(gatewayId, " ", "-", -1) // helium add dashes between three words - do this in JS to keep api generic

	networkId := request.URL.Query().Get("network_id")
	networkId, err = url.QueryUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	startTimeString := request.URL.Query().Get("start_time")
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	startTime := time.Time{}
	if startTimeString != "" {
		// parse rfc-3339 datetime
		startTime, err = time.Parse(time.RFC3339, startTimeString)
		if err != nil {
			responses.RenderError(writer, request, errors.New("can not parse start_time"))
			return
		}
	}

	endTimeString := request.URL.Query().Get("end_time")
	endTimeString, err = url.QueryUnescape(endTimeString)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	endTime := time.Now()
	if endTimeString != "" {
		// parse rfc-3339 datetime
		endTime, err = time.Parse(time.RFC3339, endTimeString)
		if err != nil {
			responses.RenderError(writer, request, errors.New("can not parse end_time"))
			return
		}
	}

	// Find all gateways with this name or ID
	gateways := database.GetGatewaysByNameOrId(gatewayId)

	// Where all results will be stored
	result := make([]responses.DeviceMeasurement, 0)

	for _, gateway := range gateways {
		gatewayDataDbRows, err := database.GetPacketsForGateway(networkId, gateway.GatewayId, startTime, endTime, 10000)
		if err != nil {
			responses.RenderError(writer, request, err)
			return
		}

		for gatewayDataDbRows.Next() {
			measurement := responses.DeviceMeasurement{}
			err = database.Db.ScanRows(gatewayDataDbRows, &measurement)
			if err != nil {
				responses.RenderError(writer, request, err)
				break
			}
			result = append(result, measurement)
		}
	}

	result = responses.AnonymiseDeviceMeasurement(result)

	render.JSON(writer, request, result)
}
