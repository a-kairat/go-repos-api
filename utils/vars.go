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
func CheckEnvVars(user, pswd, accessToken bool) {

	if user && pswd && accessToken {
		if !userOK || !pswdOK || !accessTokenOk {
			log.Printf("DBUSER: %v\t DBPASSWORD: %v\t GITHUB_ACCESS_TOKEN: %v\n", userOK, pswdOK, accessTokenOk)
			os.Exit(1)
		}
	}

	if user && !userOK {
		log.Printf("DBUSER: %v\n", userOK)
		os.Exit(1)
	}

	if pswd && !pswdOK {
		log.Printf("DBPASSWORD: %v\n", pswdOK)
		os.Exit(1)
	}

	if accessToken && !accessTokenOk {
		log.Printf("GITHUB_ACCESS_TOKEN: %v\n", accessTokenOk)
		os.Exit(1)
	}

}
