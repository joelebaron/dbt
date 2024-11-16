package dbActions

import (
	"database/sql"
	"fmt"
	db "joelebaron/dbt/packages"
	"log"

	_ "github.com/jcmturner/gokrb5/v8/iana/nametype"
)

func CopyLogins (args []string) {
	if len(args) != 5 {
		exitHelp()
	}
	sourceServer := args[2]
	targetServer := args[3]
	loginSpec := args[4]



	fmt.Println("Copying Logins: ", loginSpec , " From Server: " ,sourceServer, " To Server: ", targetServer)
	sourceConn, err := db.Connect(sourceServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", sourceServer)
		fmt.Println(err.Error())
		exitHelp()
	}
	targetConn, err := db.Connect(targetServer)
	if err != nil {
		fmt.Println("Error connection to Targer Server: ", targetServer)
		fmt.Println(err.Error())
		exitHelp()
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
		exitHelp()
	}

	for rows.Next() {

		var name string
		var command string
		var sid  string

		if err := rows.Scan(&name, &sid, &command); err != nil {
			fmt.Println("Unable to retrieve Row")
			exitHelp()
		}

		if validateNameandSid(targetConn, name, sid) {
			fmt.Println("Creating Login ", name, " on server ", targetServer)
			_, err = targetConn.Query(command)
			if err != nil {
				fmt.Println("Error creating login on server ", targetServer)
				fmt.Println(command)
				fmt.Println(err.Error())
				exitHelp()
			}
		}

	}
}

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
		exitHelp()
	}

	// If the count is > 0 the login already exists
	if rows.Next() {
		fmt.Println("Login ", name, " already exists on server")
		// check if the SID matches
		var targetName string
		var targetSid string
		if err := rows.Scan(&targetName, &targetSid); err != nil {
			fmt.Println("Unable to retrieve Row")
			exitHelp()
		}
		if targetSid != sid {
			fmt.Println("Login ", name, " SID does not match on server")
			exitHelp()
		}
		return false
	}
	return true

}



func exitHelp () {
	log.Fatal(`
	Usage:
		dbt CopyLogins <SourceServer> <TargetServer> <LoginName>
		<LoginName> can be a single login or wild card to process multiple logins.
		`)

}

