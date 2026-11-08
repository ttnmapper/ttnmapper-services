package websiteapi

import (
	"fmt"
	"net/http"
	"ttnmapper-services/internal/websiteapi/device"
	"ttnmapper-services/internal/websiteapi/experiment"
	"ttnmapper-services/internal/websiteapi/gateway"
	"ttnmapper-services/internal/websiteapi/network"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Routes() *chi.Mux {
	router := chi.NewRouter()

	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	router.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	router.Use(
		render.SetContentType(render.ContentTypeJSON),
		middleware.RealIP,
		middleware.Logger,
		middleware.Compress(5),
		//middleware.StripSlashes,
		middleware.Recoverer,
		//chiprometheus.NewMiddleware("ttnmapper-ingress-api", 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000),
	)

	// Promehteus stats
	router.Handle("/metrics", promhttp.Handler())

	// Default endpoint
	router.Get("/", HelloRoute)

	// Authenticated endpoints
	//router.Group(func(r chi.Router) {
	//	r.Use(Auth0JwtMiddleware().Handler)
	//	r.Get("/communities", GetCommunitiesHandler)
	//	r.Post("/community/data", GetCommunityData)
	//	r.Post("/community/graph", GetCommunityGraph)
	//})

	router.Mount("/gateway", gateway.Routes())
	router.Mount("/network", network.Routes())
	router.Mount("/device", device.Routes())
	router.Mount("/experiment", experiment.Routes())

	return router
}

func HelloRoute(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "TTN Mapper website API")
}
