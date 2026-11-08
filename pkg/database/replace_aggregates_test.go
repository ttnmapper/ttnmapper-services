package database

import "testing"

func TestReplacementRejectsUnsafeRowsBeforeDatabaseAccess(t *testing.T) {
	old := Db
	Db = nil
	t.Cleanup(func() { Db = old })
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"zero antenna grid", func() error { return ReplaceGridCellsForAntenna(0, nil) }},
		{"zero antenna radar", func() error { return ReplaceRadarBeamsForAntenna(0, nil) }},
		{"foreign antenna grid", func() error { return ReplaceGridCellsForAntenna(7, []GridCell{{AntennaID: 8}}) }},
		{"foreign antenna radar", func() error { return ReplaceRadarBeamsForAntenna(7, []RadarBeam{{AntennaID: 8}}) }},
		{"existing ID grid", func() error { return ReplaceGridCellsForAntenna(7, []GridCell{{AntennaID: 7, ID: 9}}) }},
		{"existing ID radar", func() error { return ReplaceRadarBeamsForAntenna(7, []RadarBeam{{AntennaID: 7, ID: 9}}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("unsafe replacement accepted")
			}
		})
	}
}
