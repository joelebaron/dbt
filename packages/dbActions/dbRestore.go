package dbActions

import (
	"database/sql"
	"flag"
	"fmt"
	db "joelebaron/dbt/packages"
	"strconv"
	"strings"
	"time"

	"log"

	_ "github.com/jcmturner/gokrb5/v8/iana/nametype"
	//"golang.org/x/text/date"
)

func DbRestore(args []string) {
	// process flag command line arguments
	sourceServer := flag.String("sourceServer", "", "Source Server Name")
	targetServer := flag.String("targetServer", "", "Target Server Name")

	sourceDB := flag.String("sourceDB", "", "Source Database Name")
	targetDB := flag.String("targetDB", "", "Target Database Name")

	backupLocationOveride := flag.String("backupLocationOveride", "", "Override Location for the Backup Files")
	replaceIfExists := flag.Bool("replaceIfExists", false, "Replace the database if it exists")
	recover := flag.Bool("recover", false, "recover the database")
	noExecute := flag.Bool("execute", false, "Execute the restore")

	/*
		applyDiff := flag.Bool("execute", false, "Apply a Difference backup when full completes")

	*/
	dataFileLocation := flag.String("dataFileLocation", "", "Override Location for the Backup Files")
	logFileLocation := flag.String("logFileLocation", "", "Override Location for the Backup Files")

	flag.CommandLine.Parse(args[2:])

	if *sourceServer == "" {
		fmt.Println("Source Server is required")
		exitHelp()
	}

	if *targetServer == "" {
		fmt.Println("Target Server is required")
		exitHelp()
	}

	if *sourceDB == "" {
		fmt.Println("Source Database is required")
		exitHelp()
	}

	if *targetDB == "" {
		*targetDB = *sourceDB
	}

	sourceConn, err := db.Connect(*sourceServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", sourceServer)
		fmt.Println(err.Error())
		exitHelp()
	}

	targetConn, err := db.Connect(*targetServer)
	if err != nil {
		fmt.Println("Error connection to Source Server: ", sourceServer)
		fmt.Println(err.Error())
		exitHelp()
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
	if result != ""	{
		databaseExists = true
	}
	if databaseExists && !*replaceIfExists {
		fmt.Println("Database ", *targetDB, " already exists on ", *targetServer)
		exitHelp()
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
		exitHelp()
	}

	var media_set_id int
	for rows.Next() {
		var backup_finish_date *time.Time

		if err := rows.Scan(&media_set_id, &backup_finish_date); err != nil {
			fmt.Println("Unable to retrieve Row")
			exitHelp()
		}

		if backup_finish_date == nil {
			fmt.Println("No Full Backup Found for Database ", *sourceDB)
			exitHelp()
		}

		//Calulate the number of days between now and backupfinishdate

		days := time.Since(*backup_finish_date).Hours() / 24

		fmt.Println("Database ", *targetDB, "Media Set Id: ", media_set_id, ".  Last Full Backup Date: ", *backup_finish_date, " (", int(days), "days ago)")
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
		exitHelp()
	}

	//iterate through the rows and add to the mediaFamily struct
	var mediaFamilies []mediaFamily
	for rows.Next() {
		var physical_device_name string
		var device_type int
		var family_sequence_number int

		if err := rows.Scan(&physical_device_name, &device_type, &family_sequence_number); err != nil {
			fmt.Println("Unable to retrieve Row")
			exitHelp()
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
		switch mediaFamily.device_type {
		case 2:
			strFiles += "\tDISK = '" + mediaFamily.physical_device_name + "',\n"
		case 9:
			strFiles += "\tURL = '" + mediaFamily.physical_device_name + "',\n"
		}
	}
	// remove the trailing comma
	strFiles = strFiles[:len(strFiles)-2] + "\n"

	var move string
	_ = move

	if databaseExists {
		move = "WITH REPLACE\n"
	} else {

		//RESTORE FILELISTONLY from the strFiles
		strSQL = "RESTORE FILELISTONLY FROM " + strFiles
		rows, err = targetConn.Query(strSQL)
		if err != nil {
			fmt.Println("Query Failed.")
			fmt.Println(strSQL)
			fmt.Println(err.Error())
			exitHelp()
		}

		// RESTORE FILELISTONLY returns different columns depending on the version of SQL Server
		// So we need to determine the columns returned and process accordingly
		columns, _ := rows.Columns()
		values := make([]interface{}, len(columns))
		scanArgs := make([]interface{}, len(columns))

		for i := range values {
			scanArgs[i] = &values[i]
		}

		move = "WITH \n"
		// iterate through the rows and output the logical name and physical name
		for rows.Next() {
			err := rows.Scan(scanArgs...)
			if err != nil {
				log.Fatal("Error scanning row: ", err)
			}

			// Process only the first two columns
			logicalName := values[0]
			ftype := values[2]
			fileID := strconv.FormatInt(values[6].(int64), 10)

			// If the type is 'D' then add the logical name to the move string
			if ftype == "D" {
				move += "MOVE '" + logicalName.(string) + "' TO '" + *dataFileLocation + *targetDB + "." + fileID + ".MDF',\n"
			}
			if ftype == "L" {
				move += "MOVE '" + logicalName.(string) + "' TO '" + *logFileLocation + *targetDB + "." + fileID + ".LDF',\n"
			}
		}

		move = move[:len(move)-2] + "\n"

	}


	if *recover {
		move += ", RECOVERY\n"
	} else {
		move += ", NORECOVERY\n"
	}

	restoreCommand := "RESTORE DATABASE " + *targetDB + " FROM"
	restoreCommand += strFiles
	restoreCommand += move

	fmt.Println(restoreCommand)

	if *noExecute {
		fmt.Println("Not Exexuting")
		return
	} else {
		_, err = targetConn.Exec(restoreCommand)
		if err != nil {
			fmt.Println("Restore Failed.")
			fmt.Println(err.Error())
			exitHelp()
		}
		fmt.Println("Restore Complete")
		return
	}


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
			mediaFamilies[i].physical_device_name = backupLocationOveride + originalName[lastSlashPos:]
		} else {
			mediaFamilies[i].physical_device_name = backupLocationOveride
		}

	}
	return mediaFamilies
}
