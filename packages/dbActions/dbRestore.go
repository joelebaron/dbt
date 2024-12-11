package dbActions

import (
	"database/sql"
	"flag"
	"fmt"
	db "joelebaron/dbt/packages/db"
	log "joelebaron/dbt/packages/log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"
	"syscall"
)

func processVars(sourceServer *string, targetServer *string, sourceDB *string, targetDB *string) {
	if *sourceServer == "" {
		fmt.Println("Source Server is required")
		log.ExitHelp("DbRestore")
	}

	if *targetServer == "" {
		fmt.Println("Target Server is required")
		log.ExitHelp("DbRestore")
	}

	if *sourceDB == "" {
		fmt.Println("Source Database is required")
		log.ExitHelp("DbRestore")
	}

	if *targetDB == "" {
		*targetDB = *sourceDB
	}

}



func DbRestore(args []string) {
	// process flag command line arguments
	sourceServer := flag.String("sourceServer", "", "Source Server Name")
	targetServer := flag.String("targetServer", "", "Target Server Name")

	sourceDB := flag.String("sourceDB", "", "Source Database Name")
	targetDB := flag.String("targetDB", "", "Target Database Name")

	backupLocationOveride := flag.String("backupLocationOveride", "", "Override Location for the Backup Files")
	replaceIfExists := flag.Bool("replaceIfExists", false, "Replace the database if it exists")
	recover := flag.Bool("recover", false, "recover the database")
	noExecute := flag.Bool("noExecute", false, "Execute the restore")

	dataFileLocation := flag.String("dataFileLocation", "", "Location to put the data files.  Defaults to the default data location")
	logFileLocation := flag.String("logFileLocation", "", "Location to put the log files.  Defaults to the default log location")

	flag.CommandLine.Parse(args[2:])

	processVars(sourceServer, targetServer, sourceDB, targetDB)


	sourceConn, err := db.Connect(*sourceServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", *sourceServer)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}

	targetConn, err := db.Connect(*targetServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", *targetServer)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}

	if *dataFileLocation == "" {
		var result string
		query := "select CAST(SERVERPROPERTY ('InstanceDefaultDataPath') as varchar(max))"
		targetConn.QueryRow(query).Scan(&result)
		dataFileLocation = &result

	}

	if *logFileLocation == "" {
		var result string
		query := "select CAST(SERVERPROPERTY ('InstanceDefaultLogPath') as varchar(max))"
		targetConn.QueryRow(query).Scan(&result)
		logFileLocation = &result

	}

	//Check if target database exists
	var result string
	databaseExists := false
	query := "select name from master.sys.databases where name = '" + *targetDB + "'"
	targetConn.QueryRow(query).Scan(&result)
	if result != "" {
		databaseExists = true
	}
	if databaseExists && !*replaceIfExists {
		fmt.Println("Database ", *targetDB, " already exists on ", *targetServer)
		log.ExitHelp("DbRestore")
	}

	strSQL := `
		SELECT top 1
			media_set_id,
			backup_finish_date
		FROM msdb.dbo.backupset
		WHERE Type = 'D' --Full
		and database_name = @dbname
		order by backup_finish_date desc
		`

	rows, err := sourceConn.Query(strSQL, sql.Named("dbname", *sourceDB))
	if err != nil {
		fmt.Println("Query Failed.")
		fmt.Println(strSQL)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}

	var media_set_id int
	for rows.Next() {
		var backup_finish_date *time.Time

		if err := rows.Scan(&media_set_id, &backup_finish_date); err != nil {
			fmt.Println("No backup found on server", sourceServer, "for database", sourceDB)
			log.ExitHelp("DbRestore")
		}

		if backup_finish_date == nil {
			fmt.Println("No bcakup found on server", sourceServer, "for database", sourceDB)
			log.ExitHelp("DbRestore")
		}

		//Calulate the number of days between now and backupfinishdate

		days := time.Since(*backup_finish_date).Hours() / 24

		fmt.Println("Database", *targetDB, "Media Set Id:", media_set_id, ".  Last Full Backup Date:", *backup_finish_date, " (", int(days), "days ago)")
	}

	strSQL = `
	SELECT physical_device_name, device_type, family_sequence_number
    FROM msdb.dbo.backupmediafamily
    WHERE media_set_id = @media_set_id
	ORDER BY family_sequence_number`

	rows, err = sourceConn.Query(strSQL, sql.Named("media_set_id", media_set_id))
	if err != nil {
		fmt.Println("Query Failed.")
		fmt.Println(strSQL)
		fmt.Println(err.Error())
		log.ExitHelp("DbRestore")
	}


	//iterate through the rows and add to the mediaFamily struct
	var mediaFamilies []mediaFamily
	for rows.Next() {
		var physical_device_name string
		var device_type int
		var family_sequence_number int

		if err := rows.Scan(&physical_device_name, &device_type, &family_sequence_number); err != nil {
			fmt.Println("Unable to retrieve Row")
			log.ExitHelp("DbRestore")
		}

		mediaFamilies = append(mediaFamilies, mediaFamily{physical_device_name, device_type, family_sequence_number})
	}

	if *backupLocationOveride != "" {
		mediaFamilies = overrideBackupLocation(*backupLocationOveride, mediaFamilies)
	}

	//output a message for the first row of mediaFamilies
	fmt.Println("First File: ", mediaFamilies[0].physical_device_name,
		" Device Type: ", mediaFamilies[0].device_type,
		" FileCount: ", mediaFamilies[len(mediaFamilies)-1].family_sequence_number)

	//concatonate the physical_device_names into a string suitable for sql restore command
	var strFiles string = "\n"
	for _, mediaFamily := range mediaFamilies {
		// if mediaFamily.physical_device_name starst with htps://
		// then use URL instead of DISK
		if strings.HasPrefix(mediaFamily.physical_device_name, "https://") {
			mediaFamily.device_type = 9
		}

		switch mediaFamily.device_type {
		case 2:
			strFiles += "\tDISK = '" + mediaFamily.physical_device_name + "',\n"
		case 9:
			strFiles += "\tURL = '" + mediaFamily.physical_device_name + "',\n"
		}
	}
	// remove the trailing comma
	strFiles = strFiles[:len(strFiles)-2] + "\n"

	var withClause string

	if databaseExists {
		withClause = "WITH REPLACE\n"
		strSQL = "ALTER DATABASE [" + *targetDB +"] SET Single_USER with rollback immediate;"

		_, err = targetConn.Exec(strSQL)
		if err != nil {
			fmt.Println("Query Failed.")
			fmt.Println(strSQL)
			fmt.Println(err.Error())
			log.ExitHelp("DbRestore")
		}



	} else {

		//RESTORE FILELISTONLY from the strFiles
		strSQL = "RESTORE FILELISTONLY FROM " + strFiles
		rows, err = targetConn.Query(strSQL)
		if err != nil {
			fmt.Println("Query Failed.")
			fmt.Println(strSQL)
			fmt.Println(err.Error())
			log.ExitHelp("DbRestore")
		}

		// RESTORE FILELISTONLY returns different columns depending on the version of SQL Server
		// So we need to determine the columns returned and process accordingly
		columns, _ := rows.Columns()
		values := make([]interface{}, len(columns))
		scanArgs := make([]interface{}, len(columns))

		for i := range values {
			scanArgs[i] = &values[i]
		}

		withClause = "WITH \n"

		for rows.Next() {
			err := rows.Scan(scanArgs...)
			if err != nil {
				fmt.Println(err.Error())
				log.ExitHelp("DbRestore")
			}

			// Process only the first two columns
			logicalName := values[0]
			ftype := values[2]
			fileID := strconv.FormatInt(values[6].(int64), 10)

			// If the type is 'D' then add the logical name to the move string
			if ftype == "D" {
				withClause += "MOVE '" + logicalName.(string) + "' TO '" + *dataFileLocation + *targetDB + "." + fileID + ".MDF',\n"
			}
			if ftype == "L" {
				withClause += "MOVE '" + logicalName.(string) + "' TO '" + *logFileLocation + *targetDB + "." + fileID + ".LDF',\n"
			}
		}

		withClause = withClause[:len(withClause)-2] + "\n"

	}

	if *recover {
		withClause += ", RECOVERY\n"
	} else {
		withClause += ", NORECOVERY\n"
	}

	restoreCommand := "RESTORE DATABASE " + *targetDB + " FROM"
	restoreCommand += strFiles
	restoreCommand += withClause

	fmt.Println(restoreCommand)
	fmt.Println("Restoring database", *targetDB, "on server", *targetServer)

	if *noExecute {
		fmt.Println("Not Executing")
		return
	} else {
		// create a channel to signal the restore process is complete
		done := make(chan bool)
		var wg sync.WaitGroup

		// Thread one will do the restore
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := targetConn.Exec(restoreCommand)

			if err != nil {
				fmt.Println("Restore Failed.")
				fmt.Println(err.Error())
				log.ExitHelp("DbRestore")
			}
			fmt.Println("\nRestore Complete")
			done <- true
		}()

		// Second thread will moniotor restore progress and output to the console
		wg.Add(1)
		go func() {
			defer wg.Done()
			pollRestoreProgress(targetConn, done, targetDB)
		}()

		wg.Wait()


		if *recover {
			fixLogins(targetConn, *targetDB)
		}

		return
	}

}

func pollRestoreProgress(targetConn *sql.DB, done chan bool, targetDB *string) {
	// Handle termination signals for clean exit
	quit := make(chan os.Signal, 1)


	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-done:
			// Restore completed
			return
		case <-quit:
			// Exit on signal
			fmt.Println("\nPolling stopped.")
			os.Exit(0)
		default:
			// Query for restore progress
			progress, estimated_completion_time, err := getRestoreProgress(targetConn, targetDB)
			if err != nil {
				fmt.Printf("Error querying restore progress: %v", err)
			} else {
				fmt.Printf("\rRestore Progress: %.2f%% - Estimated Completion: %s", progress, estimated_completion_time.Format(time.RFC1123))
			}

			// Wait for 5 seconds before the next poll
			time.Sleep(5 * time.Second)
		}
	}
}

func getRestoreProgress(targetConn *sql.DB,  targetDB *string) (float32, time.Time, error) {
	var progress float32
	var estimated_completion_time time.Time
	query := `SELECT percent_complete,
		dateadd(second,estimated_completion_time/1000, getdate()) as estimated_completion_time
	 	FROM sys.dm_exec_requests r
		CROSS APPLY sys.dm_exec_sql_text(r.sql_handle) a
		WHERE command = 'RESTORE DATABASE'
		and a.text like 'RESTORE DATABASE ` + *targetDB + `%'`
	err := targetConn.QueryRow(query).Scan(&progress, &estimated_completion_time)
	if err == sql.ErrNoRows {
		// No restore in progress
		return 0, estimated_completion_time, nil
	}
	return progress, estimated_completion_time, err
}


// create a struct for media family containing physical_device_name, device_type, family_sequence_number
type mediaFamily struct {
	physical_device_name   string
	device_type            int
	family_sequence_number int
}

func overrideBackupLocation(backupLocationOveride string, mediaFamilies []mediaFamily) []mediaFamily {
	//iterate through the mediaFamilies and replace the physical_device_name with the backupLocationOveride
	for i := range mediaFamilies {
		originalName := mediaFamilies[i].physical_device_name
		lastSlashPos := strings.LastIndexAny(originalName, "/\\")
		if lastSlashPos != -1 {
			mediaFamilies[i].physical_device_name = backupLocationOveride + originalName[lastSlashPos+1:]
		} else {
			mediaFamilies[i].physical_device_name = backupLocationOveride
		}

	}
	return mediaFamilies
}
