package store

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-gorp/gorp"
)

type teardownFunc func()

func getMockSQLStore(t *testing.T) (*SQLStore, sqlmock.Sqlmock, teardownFunc) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	store := &SQLStore{
		master: &gorp.DbMap{
			Db:            db,
			TypeConverter: mattermConverter{},
			Dialect: gorp.MySQLDialect{
				Engine:   "InnoDB",
				Encoding: "UTF8MB4",
			},
		},
	}

	teardown := func() {
		db.Close()
	}

	return store, mock, teardown
}
