package experiment

import (
	"errors"
	"net/http"
	"time"
	"ttnmapper-services/cmd/website-api/responses"
	"ttnmapper-services/pkg/database"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func Routes() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/list", GetExperimentList)
	router.Get("/find", FindExperiments)
	router.Get("/data", GetExperimentData)

	return router
}

func GetExperimentList(writer http.ResponseWriter, request *http.Request) {
	var err error

	experiments := database.GetExperimentList()
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	result := make([]responses.ExperimentResponse, 0)
	for _, experiment := range experiments {
		result = append(result, responses.ExperimentResponse{
			ID:   experiment.ID,
			Name: experiment.Name,
		})
	}

	render.JSON(writer, request, result)
}

func FindExperiments(writer http.ResponseWriter, request *http.Request) {
	var err error

	experiment := request.URL.Query().Get("experiment")
	if experiment == "" {
		responses.RenderError(writer, request, errors.New("experiment not set"))
		return
	}

	experiments := database.FindExperiment(experiment)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	result := make([]responses.ExperimentResponse, 0)
	for _, experiment := range experiments {
		result = append(result, responses.ExperimentResponse{
			ID:   experiment.ID,
			Name: experiment.Name,
		})
	}

	render.JSON(writer, request, result)
}

func GetExperimentData(writer http.ResponseWriter, request *http.Request) {
	var err error

	experiment := request.URL.Query().Get("experiment")
	if experiment == "" {
		responses.RenderError(writer, request, errors.New("experiment not set"))
		return
	}

	startTimeString := request.URL.Query().Get("start_time")
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
	endTime := time.Now()
	if endTimeString != "" {
		// parse rfc-3339 datetime
		endTime, err = time.Parse(time.RFC3339, endTimeString)
		if err != nil {
			responses.RenderError(writer, request, errors.New("can not parse end_time"))
			return
		}
	}

	experimentDataDbRows, err := database.GetPacketsForExperiment(experiment, startTime, endTime, 10000)
	if err != nil {
		responses.RenderError(writer, request, err)
		return
	}

	result := make([]responses.DeviceMeasurement, 0)
	for experimentDataDbRows.Next() {
		measurement := responses.DeviceMeasurement{}
		err = database.Db.ScanRows(experimentDataDbRows, &measurement)
		if err != nil {
			responses.RenderError(writer, request, err)
			break
		}
		result = append(result, measurement)
	}

	render.JSON(writer, request, result)
}
