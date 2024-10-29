package main
// https://github.com/microsoft/go-mssqldb#readme
// go intall dlv
// go get github.com/microsoft/go-mssqldb
// go get github.com/microsoft/go-mssqldb/integratedauth/krb5
// go get github.com/Azure/azure-sdk-for-go/sdk/azidentity


import (
		"fmt"
		_ "github.com/microsoft/go-mssqldb"
    	_ "github.com/microsoft/go-mssqldb/integratedauth/krb5"
)

func main () {
	fmt.Println("Hello")
}
