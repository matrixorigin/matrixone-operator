// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
