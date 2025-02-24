package db

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"github.com/dangthanhduong01/simplebank/db/utils"
	_ "github.com/lib/pq"
)

// var testStore Store
var testQueries *Queries
var testDB *sql.DB

func TestMain(m *testing.M) {
	config, err := utils.LoadConfig("../..")
	if err != nil {
		log.Fatal("cannot")
	}

	testDB, err = sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}
	// testStore = NewStore(testDB)
	testQueries = New(testDB)

	os.Exit(m.Run())
}
