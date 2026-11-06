package device

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"ttnmapper-services/cmd/website-api/responses"
	"ttnmapper-services/pkg/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func Routes() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/data", GetDeviceData)
	router.Get("/csv", GetDeviceCsv)

	return router
}

func GetDeviceData(writer http.ResponseWriter, request *http.Request) {
	var err error

	deviceId := request.URL.Query().Get("dev_id")
	deviceId, err = url.QueryUnescape(deviceId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	applicationId := request.URL.Query().Get("app_id")
	applicationId, err = url.QueryUnescape(applicationId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	networkId := request.URL.Query().Get("network_id")
	networkId, err = url.QueryUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	log.Println("network_id", networkId)

	limitStr := request.URL.Query().Get("limit")
	limitInt, err := strconv.ParseInt(limitStr, 10, 64)
	var limit *int64 = nil
	if err != nil {
		limit = &limitInt
	}

	startTimeString := request.URL.Query().Get("start_time")
	startTimeString, err = url.QueryUnescape(startTimeString)
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

	deviceDataDbRows, err := database.GetPacketsForDevice(networkId, applicationId, deviceId, startTime, endTime, limit)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	result := make([]responses.DeviceMeasurement, 0)
	for deviceDataDbRows.Next() {
		measurement := responses.DeviceMeasurement{}
		err = database.Db.ScanRows(deviceDataDbRows, &measurement)
		if err != nil {
			responses.RenderError(writer, request, err)
			break
		}
		result = append(result, measurement)
	}

	render.JSON(writer, request, result)
}

func GetDeviceCsv(writer http.ResponseWriter, request *http.Request) {
	//https://api.ttnmapper.org/device/csv?dev_id=cricket-002&start_time=2022-04-01&end_time=2022-04-30
	var err error

	deviceId := request.URL.Query().Get("dev_id")
	if deviceId == "" {
		responses.RenderError(writer, request, errors.New("dev_id not set"))
		return
	}
	deviceId, err = url.QueryUnescape(deviceId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	applicationId := request.URL.Query().Get("app_id")
	applicationId, err = url.QueryUnescape(applicationId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	networkId := request.URL.Query().Get("network_id")
	networkId, err = url.QueryUnescape(networkId)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}
	log.Println("network_id", networkId)

	limitStr := request.URL.Query().Get("limit")
	limitInt, err := strconv.ParseInt(limitStr, 10, 64)
	var limit *int64 = nil
	if err != nil {
		limit = &limitInt
	}

	startTimeString := request.URL.Query().Get("start_time")
	startTimeString, err = url.QueryUnescape(startTimeString)
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

	deviceDataDbRows, err := database.GetPacketsForDevice(networkId, applicationId, deviceId, startTime, endTime, limit)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	result := make([]responses.DeviceMeasurement, 0)
	for deviceDataDbRows.Next() {
		measurement := responses.DeviceMeasurement{}
		err = database.Db.ScanRows(deviceDataDbRows, &measurement)
		if err != nil {
			responses.RenderError(writer, request, err)
			break
		}
		result = append(result, measurement)
	}

	render.JSON(writer, request, result)
}
