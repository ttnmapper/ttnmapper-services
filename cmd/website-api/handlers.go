package main

import (
	"net/http"
	"ttnmapper-services/cmd/website-api/responses"

	"github.com/go-chi/render"
)

/*
// Get data
GetCommunities for user
If specified community in list continue, else no permission
Get tariffs

*/

func GetGatewayRadar(w http.ResponseWriter, r *http.Request) {
	errorResponse := responses.ErrorResponse{}

	//if err != nil {
	errorResponse.Success = false
	errorResponse.Message = "can't determine userid"
	//log.Print(err.Error())
	render.Status(r, http.StatusUnauthorized)
	render.JSON(w, r, errorResponse)
	return
	//}

	render.Status(r, http.StatusOK)
	//render.JSON(w, r, communities)
}
