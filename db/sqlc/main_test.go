package db

import (
	"context"
	"log"
	"os"
	"testing"

	gf "github.com/brianvoe/gofakeit/v6"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/niiniyare/awo/util"
)

var testQueries *Queries
var testDB *pgxpool.Pool

var fk *gf.Faker

func TestMain(m *testing.M) {
	config, err := util.LoadConfig("../..")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	testDB, err = pgxpool.Connect(context.Background(), config.DBSource)
	if err != nil {
		log.Fatalln("testDB failed to connect:", err)
	}

	defer testDB.Close()
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}

	testQueries = New(testDB)

	fk = gf.NewCrypto()

	os.Exit(m.Run())
}
