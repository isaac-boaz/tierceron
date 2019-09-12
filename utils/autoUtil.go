package utils

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	sys "bitbucket.org/dexterchaney/whoville/vaulthelper/system"

	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
	"gopkg.in/yaml.v2"
)

type cert struct {
	ApproleID string `yaml:"approleID"`
	SecretID  string `yaml:"secretID"`
}

func (c *cert) getCert() *cert {
	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Printf("User home directory #%v ", err)
	}

	yamlFile, err := ioutil.ReadFile(userHome + "/.vault/configcert.yml")
	if err != nil {
		log.Printf("yamlFile.Get err #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}

// AutoAuth attempts to
func AutoAuth(secretIDPtr *string, appRoleIDPtr *string, tokenPtr *string, tokenNamePtr *string, envPtr *string, addrPtr *string) {
	// Declare local variables
	var override bool
	var exists bool
	var c cert

	// Get current user's home directory
	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Printf("User home directory #%v ", err)
	}

	// New values available for the cert file
	if *secretIDPtr != "" && *appRoleIDPtr != "" {
		override = true
	}

	// If cert file exists obtain secretID and appRoleID
	if *tokenPtr == "" {
		if _, err := os.Stat(userHome + "/.vault/configcert.yml"); !os.IsNotExist(err) {
			exists = true
			if !override {
				fmt.Println("Grabbing config IDs from cert file.")
				c.getCert()
				*secretIDPtr = c.SecretID
				*appRoleIDPtr = c.ApproleID
			}
		}
	}

	// Overriding or first time access: request IDs and create cert file
	if *tokenPtr == "" && (override || !exists) {
		scanner := bufio.NewScanner(os.Stdin)
		var secretID string
		var approleID string
		var dump []byte

		if override {
			fmt.Println("Overriding cert file with new config IDs")
		} else {

			// Enter ID tokens
			fmt.Println("No cert file found, please enter config IDs")
			fmt.Print("secretID: ")
			scanner.Scan()
			secretID = scanner.Text()
			fmt.Print("approleID: ")
			scanner.Scan()
			approleID = scanner.Text()
		}

		// Get dump
		if override && exists {
			fmt.Printf("Creating new cert file in %s: secretID has been set to %s, approleID has been set to %s\n", userHome+"/.vault/configcert.yml", *secretIDPtr, *appRoleIDPtr)
			dump = []byte("approleID: " + *appRoleIDPtr + "\nsecretID: " + *secretIDPtr)
		} else if override && !exists {
			fmt.Println("No cert file exists, continuing without saving config IDs")
		} else {
			fmt.Printf("Creating cert file in %s: secretID has been set to %s, approleID has been set to %s\n", userHome+"/.vault/configcert.yml", secretID, approleID)
			dump = []byte("approleID: " + approleID + "\nsecretID: " + secretID)
		}

		// Do not save IDs if overriding and no cert file exists
		if !override || exists {

			// Create hidden folder
			if _, err := os.Stat(userHome + "/.vault"); os.IsNotExist(err) {
				err = os.MkdirAll(userHome+"/.vault", 0700)
				if err != nil {
					log.Fatal(err)
				}
			}

			// Create cert file
			writeErr := ioutil.WriteFile(userHome+"/.vault/configcert.yml", dump, 0600)
			if writeErr != nil {
				fmt.Printf("Unable to write file: %v\n", writeErr)
			}
		}

		// Set config IDs
		if !override {
			*secretIDPtr = secretID
			*appRoleIDPtr = approleID
		}

		// Checks that the scanner is working
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	//if using appRole
	if *secretIDPtr != "" || *appRoleIDPtr != "" || *tokenNamePtr != "" {
		switch *envPtr {
		case "dev":
			*tokenNamePtr = "config_token_dev"
		case "QA":
			*tokenNamePtr = "config_token_QA"
		case "RQA":
			*tokenNamePtr = "config_token_RQA"
		case "itdev":
			*tokenNamePtr = "config_token_itdev"
		case "servicepack":
			*tokenNamePtr = "config_token_servicepack"
		case "local":
			*tokenNamePtr = "config_token_local"
		case "staging":
			*tokenNamePtr = "config_token_staging"
		}
		//check that none are empty
		if *secretIDPtr == "" {
			CheckWarning(fmt.Sprintf("Missing required secretID"), true)
		} else if *appRoleIDPtr == "" {
			CheckWarning(fmt.Sprintf("Missing required appRoleID"), true)
		} else if *tokenNamePtr == "" {
			CheckWarning(fmt.Sprintf("Missing required tokenName"), true)
		}
		//check that token matches environment
		tokenParts := strings.Split(*tokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if *envPtr != tokenEnv {
			CheckWarning(fmt.Sprintf("Token doesn't match environment"), true)
		}
	}

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			CheckError(fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}
		v, err := sys.NewVault(*addrPtr)
		CheckError(err, true)

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		CheckError(err, true)

		mod, err := kv.NewModifier(master, *addrPtr)
		CheckError(err, true)
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		CheckError(err, true)
	}
}