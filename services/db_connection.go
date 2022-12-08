package services

import (
	"database/sql"
	"log"
)

const (
	DB_PORT     = "3306" //MYSQL default PORT is 3306.
	DB_USER     = "asterisk"
	DB_PASSWORD = "Test123!"
)

var DBConnector DBCONNECTION

type DBCONNECTION struct {
	DB *sql.DB
}

type QueryResult struct {
	param1 sql.NullString
	param2 sql.NullString
	param3 sql.NullString
	//param4 sql.NullString
}

func (d *DBCONNECTION) OpenDB(dbName string) {

	var error error
	d.DB, error = sql.Open("mysql", DB_USER+":"+DB_PASSWORD+"@"+CONNECTION_TYPE+"("+SERVER_IP+":"+DB_PORT+")/"+dbName)
	if error != nil {
		log.Println("Failed to open DB: ", error.Error())
	}
}

func (d *DBCONNECTION) ExecuteQuery(dbName string, queryString string) *sql.Rows {

	d.OpenDB(dbName)

	queryResults, error := d.DB.Query(queryString)
	if error != nil {
		log.Println("DB Query Error: ", error.Error(), queryString)
	}

	d.DB.Close()

	return queryResults
}

func (d *DBCONNECTION) CreateDB(dbName string) {

	var err error
	var db *sql.DB

	if db, err = sql.Open("mysql", DB_USER+":"+DB_PASSWORD+"@"+CONNECTION_TYPE+"("+SERVER_IP+":"+DB_PORT+")/"); err != nil {
		log.Println("Error when opening database ", err)
		return
	}

	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS " + dbName); err != nil {
		log.Println("Error when creating database ", err)
		return
	}

	db.Close()
}

func (d *DBCONNECTION) CreateTable(dbName string, query string) {

	d.OpenDB(dbName)

	var err error
	if _, err = d.DB.Exec(query); err != nil {
		log.Println("Error when creating a new table", err)
	}

	d.DB.Close()
}
