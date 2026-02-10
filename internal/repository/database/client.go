package database

import "github.com/jmoiron/sqlx"

var client *sqlx.DB

func NewPostgresClient(dsn string) error {
	var err error

	client, err = sqlx.Connect("postgres", dsn)
	if err != nil {
		return err
	}
	return nil
}

func Client() *sqlx.DB {
	return client
}
