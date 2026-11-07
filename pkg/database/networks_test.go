package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"reflect"
	"strings"
	"testing"
)

// DryRun constructs PostgreSQL queries without opening a database connection.
func TestNetworkQueries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		run      func()
		fragment string
	}{
		{"peered", func() { GetPeeredNetworks("network' OR true --") }, "uplink_application_data = true"},
		{"subscription", func() { GetNetworkSubscription("network' OR true --") }, "network_id = $1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=unused user=unused dbname=unused"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			old := Db
			Db = db
			t.Cleanup(func() { Db = old })
			var query string
			var args []interface{}
			err = db.Callback().Query().After("gorm:query").Register("test:capture", func(tx *gorm.DB) { query = tx.Statement.SQL.String(); args = append(args, tx.Statement.Vars...) })
			if err != nil {
				t.Fatal(err)
			}
			tc.run()
			if !strings.Contains(query, tc.fragment) || strings.Contains(query, "OR true") {
				t.Fatalf("query = %s", query)
			}
			if !reflect.DeepEqual(args, []interface{}{"network' OR true --"}) {
				t.Fatalf("bound args = %#v", args)
			}
		})
	}
}
