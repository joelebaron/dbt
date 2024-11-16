package main

// https://github.com/microsoft/go-mssqldb#readme
// go intall dlv
// go get github.com/microsoft/go-mssqldb
// go get github.com/microsoft/go-mssqldb/integratedauth/krb5
// go get github.com/Azure/azure-sdk-for-go/sdk/azidentity

import (
	"fmt"
	"joelebaron/dbt/packages/dbActions"
	"os"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/microsoft/go-mssqldb/integratedauth/krb5"
)

func main () {
	switch strings.ToLower(os.Args[1]) {
	case "copylogins":
			dbActions.CopyLogins(os.Args)


	default:
		fmt.Println( `Usage:
	dbt CopyLogins ...
	`)
	}

}
