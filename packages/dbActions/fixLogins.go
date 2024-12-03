package dbActions

import (
	"database/sql"
	"fmt"
	log "joelebaron/dbt/packages/log"
)

func fixLogins(targetConn *sql.DB, dbName string) {
	fmt.Println("Syncing All database users with Logins")

	strSQL := "USE " + dbName

	_, err := targetConn.Exec(strSQL)
	if err != nil {
		fmt.Println("Trying to Fix Logins", dbName)
		fmt.Println("Error switching to database ", dbName)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}

	strSQL = `Declare @dname sysname
	Declare @strSQL nvarchar(max)
	DECLARE user_update CURSOR FOR
	SELECT d.name, 'EXEC sp_change_users_login ''Update_One'', ''' +d.name + ''', ''' +d.name + '''' as strSQL
	from sys.database_principals d
	join master.sys.server_principals s
		ON d.name = s.name
		and d.sid <> s.sid
	and d.type <> 'r'


	OPEN user_update;
	FETCH NEXT FROM user_update INTO @dname, @strSQL;

	WHILE @@FETCH_STATUS = 0
	BEGIN
		print 'Fixing Login / User ' + @dname
		print @strSQL
		exec (@strSQL)

		FETCH NEXT FROM user_update INTO @dname, @strSQL;
	END

	Close user_update;
	Deallocate user_update;
	`

	_, err = targetConn.Exec(strSQL)
	if err != nil {
		fmt.Println("Trying to Fix Logins", dbName)
		fmt.Println("Error Runnnig Fix logins Script ", dbName)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}

}
