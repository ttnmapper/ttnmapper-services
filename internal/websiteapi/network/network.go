package network

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"ttnmapper-services/internal/websiteapi/responses"
	"ttnmapper-services/pkg/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/j4/gosm"
	"github.com/patrickmn/go-cache"
)

var (
	// Caches for dynamic content, needs to expire
	networkZ5GatewaysCache *cache.Cache
)

func Routes() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/{network_id}/gateways", GetGateways)
	router.Get("/{network_id}/gateways/{page}", GetGatewaysPaged)
	router.Get("/{network_id}/gateways/z5tile/{x}/{y}", GetGatewaysInZ5Tile)
	router.Get("/{network_id}/gateways/bbox/{north}/{south}/{west}/{east}", GetGatewaysInBBox)

	networkZ5GatewaysCache = cache.New(4*time.Hour, 1*time.Hour)

	return router
}

func GetGateways(writer http.ResponseWriter, request *http.Request) {
	var err error

	networkId := chi.URLParam(request, "network_id")
	networkId, err = url.PathUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	// Include all gateways from networks that feed data into this network via the packet broker
	peeredNetworks := database.GetPeeredNetworks(networkId)

	responseGateways := make([]responses.Gateway, 0)

	selfInPeeredNetworks := false
	for _, network := range peeredNetworks {
		networkSubscription := database.GetNetworkSubscription(network.ForwarderNetworkId)

		if network.ForwarderNetworkId == networkId {
			selfInPeeredNetworks = true
		}
		dbGateways := database.GetOnlineGatewaysForNetwork(network.ForwarderNetworkId)

		for _, dbGateway := range dbGateways {
			if dbGateway.Latitude == 0 && dbGateway.Longitude == 0 {
				continue
			}
			gateway := responses.DbGatewayWithBoundingBoxToResponse(dbGateway)
			gateway = ApplySubscription(gateway, networkSubscription)
			responseGateways = append(responseGateways, gateway)
		}
	}

	// If no routing policies exist for this network, no gateways will be returned. So fetch for this specific network then.
	if !selfInPeeredNetworks {
		networkSubscription := database.GetNetworkSubscription(networkId)

		dbGateways := database.GetOnlineGatewaysForNetwork(networkId)
		for _, dbGateway := range dbGateways {
			if dbGateway.Latitude == 0 && dbGateway.Longitude == 0 {
				continue
			}
			gateway := responses.DbGatewayWithBoundingBoxToResponse(dbGateway)
			gateway = ApplySubscription(gateway, networkSubscription)
			responseGateways = append(responseGateways, gateway)
		}
	}

	render.JSON(writer, request, responseGateways)
}

func ApplySubscription(gateway responses.Gateway, subscription database.NetworkSubscription) responses.Gateway {
	// only include name and description if network has a subscription
	if subscription.ID == 0 || !subscription.GatewayNames {
		networkIdPrefix, _, _ := strings.Cut(gateway.NetworkId, ":")
		gateway.NetworkId = networkIdPrefix
		gateway.GatewayId = ""
		gateway.Name = ""
	}

	if subscription.ID == 0 || !subscription.GatewayDescriptions {
		if _, ok := gateway.Attributes["description"]; ok {
			gateway.Attributes["description"] = ""
		}
	}

	return gateway
}

func GetGatewaysPaged(writer http.ResponseWriter, request *http.Request) {
	var err error
	var pageLimit = 10000

	networkId := chi.URLParam(request, "network_id")
	networkId, err = url.PathUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	page := chi.URLParam(request, "page")
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		log.Println(err.Error())
		pageInt = 0
	}

	dbGateways := database.GetOnlineGatewaysForNetwork(networkId)
	pageStart := pageInt * pageLimit
	pageEnd := pageStart + pageLimit
	log.Println(len(dbGateways), pageStart, pageEnd)
	if pageStart > len(dbGateways) {
		pageStart = len(dbGateways)
	}
	if pageEnd > len(dbGateways) {
		pageEnd = len(dbGateways)
	}

	dbGateways = dbGateways[pageStart:pageEnd]
	responseGateways := responses.DbGatewaysWithBoundingBoxToResponse(dbGateways)
	render.JSON(writer, request, responseGateways)
}

func GetGatewaysInZ5Tile(writer http.ResponseWriter, request *http.Request) {
	var err error

	networkId := chi.URLParam(request, "network_id")
	networkId, err = url.PathUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	x, err := strconv.Atoi(chi.URLParam(request, "x"))
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	y, err := strconv.Atoi(chi.URLParam(request, "y"))
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	cacheIndex := networkId + "/" + strconv.Itoa(x) + "/" + strconv.Itoa(y)
	if cacheGateways, ok := networkZ5GatewaysCache.Get(cacheIndex); ok {
		render.JSON(writer, request, cacheGateways.([]responses.Gateway))
		return
	}

	tile := gosm.NewTileWithXY(x, y, 5)
	north := tile.Lat
	west := tile.Long
	tile = gosm.NewTileWithXY(x+1, y+1, 5)
	south := tile.Lat
	east := tile.Long

	dbGateways := database.GetOnlineGatewaysForNetworkInBbox(networkId, west, east, north, south)
	responseGateways := responses.DbGatewaysToResponse(dbGateways)

	networkZ5GatewaysCache.Set(cacheIndex, responseGateways, cache.DefaultExpiration)

	render.JSON(writer, request, responseGateways)
}

func GetGatewaysInBBox(writer http.ResponseWriter, request *http.Request) {
	var err error

	networkId := chi.URLParam(request, "network_id")
	networkId, err = url.PathUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	north, err := strconv.ParseFloat(chi.URLParam(request, "north"), 64)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	south, err := strconv.ParseFloat(chi.URLParam(request, "south"), 64)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	west, err := strconv.ParseFloat(chi.URLParam(request, "west"), 64)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	east, err := strconv.ParseFloat(chi.URLParam(request, "east"), 64)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	// Include all gateways from networks that feed data into this network via the packet broker
	peeredNetworks := database.GetPeeredNetworks(networkId)

	responseGateways := make([]responses.Gateway, 0)

	selfInPeeredNetworks := false
	for _, network := range peeredNetworks {
		if network.ForwarderNetworkId == networkId {
			selfInPeeredNetworks = true
		}
		dbGateways := database.GetOnlineGatewaysForNetworkInBbox(networkId, west, east, north, south)
		responseGateways = append(responseGateways, responses.DbGatewaysToResponse(dbGateways)...)
	}

	// If not routing policies exist for this network, no gateways will be returned. So fetch for this specific network then.
	if !selfInPeeredNetworks {
		dbGateways := database.GetOnlineGatewaysForNetworkInBbox(networkId, west, east, north, south)
		responseGateways = append(responseGateways, responses.DbGatewaysToResponse(dbGateways)...)
	}

	render.JSON(writer, request, responseGateways)

}
