package aggregations

import (
	"log"
	"time"
	"ttnmapper-services/pkg/aggregations/grid_cell"
	"ttnmapper-services/pkg/aggregations/radar_beam"
	"ttnmapper-services/pkg/database"
	"ttnmapper-services/pkg/types"
)

func AggregateNewData(message types.TtnMapperUplinkMessage) {
	grid_cell.AggregateNewData(message)
	radar_beam.AggregateNewData(message)
}

func AggregateMovedGateway(message types.TtnMapperGatewayMoved) {
	grid_cell.AggregateMovedGateway(message)
	radar_beam.AggregateMovedGateway(message)
}

func ReprocessAntenna(antenna database.Antenna, movedTime time.Time) {
	if err := grid_cell.ReprocessAntenna(antenna, movedTime); err != nil {
		log.Print(err)
		return
	}
	if err := radar_beam.ReprocessAntenna(antenna, movedTime); err != nil {
		log.Print(err)
	}
}
