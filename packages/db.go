package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
)


func Connect(ServerName string) (*sql.DB, error) {
	// connect to a sql server named instance
	// connString := fmt.Sprintf("sqlserver://sa:Mar32Art$@%s?database=master", ServerName )
	var instanceName string
	if strings.Contains(ServerName, "\\") {
		//Server name should be everything up to the backslash
		instanceName = strings.Split(ServerName, "\\")[1]
		ServerName = strings.Split(ServerName, "\\")[0]
	}

	connString := fmt.Sprintf("sqlserver://@%s", ServerName )
	// is instanceName != "" append the instanceName to the connection string
	if instanceName != "" {
		connString = connString + "/" + instanceName
	}

	dbConn,err := sql.Open("sqlserver",connString)

	if err != nil {
		return dbConn, err
	}

	err=dbConn.Ping()
	if err != nil {
		fmt.Println("Error pinging Source Server: ", connString)
		fmt.Println(err.Error())
	}

	return dbConn, err
}


