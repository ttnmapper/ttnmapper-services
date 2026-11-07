package database

import "gorm.io/gorm/clause"

func GetGridCell(indexer GridCellIndexer) (GridCell, error) {
	var gridCell GridCell
	gridCell.AntennaID = indexer.AntennaID
	gridCell.X = indexer.X
	gridCell.Y = indexer.Y
	err := Db.FirstOrCreate(&gridCell, &gridCell).Error
	return gridCell, err
}

func SaveGridCell(gridCell GridCell) error {
	return Db.Save(&gridCell).Error
}

func GetGridcellsForAntenna(antenna Antenna) []GridCell {
	var gridCells []GridCell
	Db.Where("antenna_id = ?", antenna.ID).Find(&gridCells)
	return gridCells
}

func CreateGridCells(gridCells []GridCell) error {
	// On conflict override
	tx := Db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&gridCells)
	return tx.Error
}

func DeleteGridCellsForAntenna(antenna Antenna) {
	Db.Where(&GridCell{AntennaID: antenna.ID}).Delete(&GridCell{})
}
