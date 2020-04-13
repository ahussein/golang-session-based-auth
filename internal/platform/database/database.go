package database

import (
	"database/sql"
	"fmt"
	"time"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/pkg/errors"

	// these are not explicitly used but need to be imported
	_ "github.com/lib/pq"
)

type (
	// ConnectionParams are the parameters used to connect to a database
	ConnectionParams struct {
		Driver               string
		Username             string
		Password             string
		Host                 string
		Port                 int
		Database             string
		MaxDBConnections     int
		MaxDBIdleConnections int
	}
)

// DataSourceName formats the parameters and returns a DSN string
func (p ConnectionParams) DataSourceName() string {
	switch p.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			p.Username,
			p.Password,
			p.Host,
			p.Port,
			p.Database,
		)
	case "postgres":
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			p.Username,
			p.Password,
			p.Host,
			p.Port,
			p.Database,
		)
	default:
		return ""
	}
}

// DB returns a connection to a database
func DB(p ConnectionParams) (*sql.DB, error) {
	dn, err := traceableDB(p.Driver)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(dn, p.DataSourceName())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open a DB connection to host %s", p.Host)
	}

	// Set max open/idle connections
	db.SetMaxOpenConns(p.MaxDBConnections)
	db.SetMaxIdleConns(p.MaxDBIdleConnections)

	retries := 10
	for retries > 0 {
		err = db.Ping()
		if err != nil {
			time.Sleep(time.Second)
			retries--
		} else {
			break
		}
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ping database %s", p.Host)
	}

	// record db connection pool stats
	ocsql.RecordStats(db, 5*time.Second)
	return db, nil
}

func traceableDB(driver string) (string, error) {
	dn, err := ocsql.Register(driver, ocsql.WithOptions(ocsql.TraceOptions{
		AllowRoot:    true,
		RowsAffected: true,
		LastInsertID: true,
		Query:        true,
	}))
	if err != nil {
		return "", err
	}
	return dn, nil
}
