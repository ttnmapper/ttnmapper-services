package database

import "gorm.io/gorm"

// RecordAntennaFrequency preserves the existing upsert/counting behavior.
func RecordAntennaFrequency(conn *gorm.DB, antennaID, frequencyID uint) error {
	return conn.Exec("INSERT INTO antenna_frequencies (antenna_id, frequency_id, count, last_heard) "+
		"VALUES (?, ?, 1, NOW()) ON CONFLICT (antenna_id, frequency_id) DO "+
		"UPDATE SET count = antenna_frequencies.count + 1, last_heard = NOW() "+
		"WHERE antenna_frequencies.antenna_id = ? AND antenna_frequencies.frequency_id = ?",
		antennaID, frequencyID, antennaID, frequencyID).Error
}
