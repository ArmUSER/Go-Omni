package db

import (
	"database/sql"
	"log"
)

var DBConnector DBCONNECTION

type DBCONNECTION struct {
	DB *sql.DB
}

type QueryResult struct {
	Param1 sql.NullString
	Param2 sql.NullString
}

func (d *DBCONNECTION) OpenDB(dbCredentials string, dbName string) {

	var error error
	d.DB, error = sql.Open("mysql", dbCredentials+"/"+dbName)
	if error != nil {
		log.Println("Failed to open DB: ", error.Error())
	}
}

func (d *DBCONNECTION) ExecuteQuery(dbCredentials string, dbName string, queryString string) *sql.Rows {

	d.OpenDB(dbCredentials, dbName)
	defer d.DB.Close()

	queryResults, error := d.DB.Query(queryString)

	if error != nil {
		log.Println("DB Query Error: ", error.Error(), queryString)
	}

	return queryResults
}

func (d *DBCONNECTION) CreateDB(dbCredentials string, dbName string) {

	var err error
	var db *sql.DB

	if db, err = sql.Open("mysql", dbCredentials+"/"); err != nil {
		log.Println("Error when opening database ", err)
		return
	}

	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS " + dbName); err != nil {
		log.Println("Error when creating database ", err)
		return
	}

	db.Close()
}

func (d *DBCONNECTION) CreateTable(dbCredentials string, dbName string, query string) {

	d.OpenDB(dbCredentials, dbName)

	var err error
	if _, err = d.DB.Exec(query); err != nil {
		log.Println("Error when creating a new table", err)
	}

	d.DB.Close()
}
