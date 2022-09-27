package sql

import (
	"database/sql"

	// mysql imports mysql driver
	_ "github.com/go-sql-driver/mysql"
)

func MySQLDialectSmokeTest(dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	queries := []string{
		"CREATE DATABASE IF NOT EXISTS test;",
		"CREATE TABLE IF NOT EXISTS test.test(id INT PRIMARY KEY);",
		"INSERT INTO test.test (id) VALUES (10000), (10086);",
		"SELECT * FROM test.test;",
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
