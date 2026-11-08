package database

import (
	"errors"
	"gorm.io/gorm"
)

// ReplaceGridCellsForAntenna commits the complete replacement or keeps the old
// rows. Empty input deliberately clears the antenna after a successful replay.
func ReplaceGridCellsForAntenna(antennaID uint, cells []GridCell) error {
	if antennaID == 0 {
		return errors.New("antenna ID must be nonzero")
	}
	for _, cell := range cells {
		if cell.AntennaID != antennaID || cell.ID != 0 {
			return errors.New("replacement cells must belong to the antenna and have no existing ID")
		}
	}
	return Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("antenna_id = ?", antennaID).Delete(&GridCell{}).Error; err != nil {
			return err
		}
		if len(cells) == 0 {
			return nil
		}
		return tx.Create(&cells).Error
	})
}

// ReplaceRadarBeamsForAntenna has the same atomic replacement semantics for radar.
func ReplaceRadarBeamsForAntenna(antennaID uint, beams []RadarBeam) error {
	if antennaID == 0 {
		return errors.New("antenna ID must be nonzero")
	}
	for _, beam := range beams {
		if beam.AntennaID != antennaID || beam.ID != 0 {
			return errors.New("replacement beams must belong to the antenna and have no existing ID")
		}
	}
	return Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("antenna_id = ?", antennaID).Delete(&RadarBeam{}).Error; err != nil {
			return err
		}
		if len(beams) == 0 {
			return nil
		}
		return tx.Create(&beams).Error
	})
}
