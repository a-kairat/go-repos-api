package utils

import (
	"log"
	"os"
)

var (
	// DBUSER is db user
	DBUSER, userOK = os.LookupEnv("DBUSER")
	// DBPSWD is db password
	DBPSWD, pswdOK = os.LookupEnv("DBPASSWORD")
	// ACCESSTOKEN is github access token
	ACCESSTOKEN, accessTokenOk = os.LookupEnv("GITHUB_ACCESS_TOKEN")
)

// CheckEnvVars checks if ENV vars are set. If not exits process.
func CheckEnvVars() {
	if !userOK || !pswdOK || !accessTokenOk {
		log.Printf("DBUSER: %v\t DBPASSWORD: %v\t GITHUB_ACCESS_TOKEN: %v\n", userOK, pswdOK, accessTokenOk)
		os.Exit(1)
	}
}
