package log

import (
	"log"
)

func ExitHelp(module string) {
	switch module {
	case "CopyLogins":
		log.Fatal(`
		Usage:
			dbt CopyLogins <SourceServer> <TargetServer> <LoginName>
			<LoginName> can be a single login or wild card to process multiple logins.
			`)
	case "DbRestore":
		log.Fatal(`
		Usage:
			dbt DbRestore
			flags:
				-sourceServer (required)
				-targetServer (required)
				-sourceDB (required)
				-targetDB (default = sourceDB)
				-backupLocationOveride (default = "") - Overrides the directory of the backup files
				-replaceIfExists (default = false) - If true, the database will be replaced if exists or restored
					to specified file locations
				-recover (default = false) - If true, the database will be recovered
				-noExecute (default = false) - If true, outputs the restore statement without executing
				-dataFileLocation (default = "") - The target servers default location will be used
				-logFileLocation (default = "") - The target servers default location will be used
				`)
	}

}
