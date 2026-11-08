package database

func GetRadarBeam(indexer RadarBeamIndexer) (RadarBeam, error) {
	var radarBeam RadarBeam
	radarBeam.AntennaID = indexer.AntennaID
	radarBeam.Level = indexer.Level
	radarBeam.Bearing = indexer.Bearing
	err := Db.FirstOrCreate(&radarBeam, &radarBeam).Error
	return radarBeam, err
}

func SaveRadarBeam(radarBeam RadarBeam) error {
	return Db.Save(&radarBeam).Error // this could happen while a reprocess for the same gateway is happening, causing a unique index issue
}

func GetRadarBeamsForAntenna(antenna Antenna) []RadarBeam {
	var radarBeams []RadarBeam
	Db.Where("antenna_id = ?", antenna.ID).Find(&radarBeams)
	return radarBeams
}
