package dbActions

import (
	"fmt"
	db "joelebaron/dbt/packages/db"
	log "joelebaron/dbt/packages/log"

	_ "github.com/jcmturner/gokrb5/v8/iana/nametype"
)

func CopyLogins (args []string) {
	if len(args) != 5 {
		log.ExitHelp("CopyLogins")
	}
	sourceServer := args[2]
	targetServer := args[3]
	loginSpec := args[4]



	fmt.Println("Copying Logins: ", loginSpec , " From Server: " ,sourceServer, " To Server: ", targetServer)
	sourceConn, err := db.Connect(sourceServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", sourceServer)
		fmt.Println(err.Error())
		log.ExitHelp("CopyLogins")
	}
	targetConn, err := db.Connect(targetServer)
	if err != nil {
		fmt.Println("Error connection to Targer Server: ", targetServer)
		fmt.Println(err.Error())
		log.ExitHelp("CopyLogins")
	}

	loginQuery := `SELECT name, sid, 'CREATE LOGIN [' + name + ']
	WITH
	PASSWORD = ' + CONVERT(NVARCHAR(MAX), password_hash, 1) + ' HASHED
	,SID = ' + CONVERT(NVARCHAR(MAX), sid, 1) + ''
	AS CreateLoginStatement
	FROM master.sys.sql_logins
	WHERE name LIKE '` + loginSpec + "'"


	rows, err := sourceConn.Query(loginQuery)
	if err != nil {
		fmt.Println("Login Query Failed.")
		fmt.Println(loginQuery)
		fmt.Println(err.Error())
		log.ExitHelp("CopyLogins")
	}

	for rows.Next() {

		var name string
		var command string
		var sid  string

		if err := rows.Scan(&name, &sid, &command); err != nil {
			fmt.Println("Unable to retrieve Row")
			log.ExitHelp("CopyLogins")
		}
		command = "IF EXISTS (SELECT name FROM master.sys.server_principals	WHERE name = '" + name + "') DROP LOGIN [" + name + "]; \n" + command
		fmt.Println(command)
		fmt.Println("Creating Login ", name, " on server ", targetServer)
		_, err = targetConn.Query(command)
		if err != nil {
			fmt.Println("Error creating login on server ", targetServer)
			fmt.Println(command)
			fmt.Println(err.Error())
			log.ExitHelp("CopyLogins")
		}
	}
}

/*
func validateNameandSid (conn *sql.DB, name string, sid string) bool {



	// Check if login exists on target server
	loginQuery := `SELECT name, sid
	FROM master.sys.sql_logins
	WHERE name = '` + name + "'"

	rows, err := conn.Query(loginQuery)
	if err != nil {
		fmt.Println("Login Query Failed.")
		fmt.Println(loginQuery)
		fmt.Println(err.Error())
		log.ExitHelp("CopyLogins")
	}

	// If the count is > 0 the login already exists
	if rows.Next() {
		fmt.Println("Login ", name, " already exists on server")
		// check if the SID matches
		var targetName string
		var targetSid string
		if err := rows.Scan(&targetName, &targetSid); err != nil {
			fmt.Println("Unable to retrieve Row")
			log.ExitHelp("CopyLogins")
		}
		if targetSid != sid {
			fmt.Println("Login ", name, " SID does not match on server")
			log.ExitHelp("CopyLogins")
		}
		return false
	}
	return true

}

*/
