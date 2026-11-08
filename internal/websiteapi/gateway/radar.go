package gateway

import (
	"log"
	"net/http"
	"net/url"
	"time"
	"ttnmapper-services/internal/websiteapi/responses"
	"ttnmapper-services/pkg/layers/radar"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func GetGatewayRadarMulti(w http.ResponseWriter, r *http.Request) {
	errorResponse := responses.ErrorResponse{}
	var err error

	networkId := chi.URLParam(r, "network_id")
	networkId, err = url.PathUnescape(networkId)
	gatewayId := chi.URLParam(r, "gateway_id")
	gatewayId, err = url.PathUnescape(gatewayId)
	log.Println(networkId, gatewayId)

	geoJson := radar.GenerateRadarMulti(networkId, gatewayId)
	if geoJson == nil {
		errorResponse.Success = false
		errorResponse.Message = "could not generate geojson"
		render.JSON(w, r, errorResponse)
		return
	}

	// Set cache headers in the response
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(geoJson)
	if err != nil {
		log.Println("could not write geojson to response")
		return
	}
}

func GetGatewayRadarSingle(w http.ResponseWriter, r *http.Request) {
	errorResponse := responses.ErrorResponse{}
	var err error

	networkId := chi.URLParam(r, "network_id")
	networkId, err = url.PathUnescape(networkId)
	gatewayId := chi.URLParam(r, "gateway_id")
	gatewayId, err = url.PathUnescape(gatewayId)
	log.Println(networkId, gatewayId)

	geoJson := radar.GenerateRadarSingle(networkId, gatewayId)
	if geoJson == nil {
		errorResponse.Success = false
		errorResponse.Message = "could not generate geojson"
		render.JSON(w, r, errorResponse)
		return
	}

	// Set cache headers in the response
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(geoJson)
	if err != nil {
		log.Println("could not write geojson to response")
		return
	}
}
