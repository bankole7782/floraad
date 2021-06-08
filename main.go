package main

import (
	"fmt"
	"os"
	"path/filepath"
	// "os/exec"
	"encoding/json"
	"github.com/pkg/errors"
	// "strings"
	"github.com/gookit/color"
)


func init() {
  rootPath, err := GetRootPath()
  if err != nil {
    panic(err)
  }
	os.MkdirAll(filepath.Join(rootPath, "p"), 0777)
	os.MkdirAll(filepath.Join(rootPath, "flotmp"), 0777)	
	os.MkdirAll(filepath.Join(rootPath, "pd"), 0777)	
}


func main() {
  rootPath, err := GetRootPath()
  if err != nil {
    panic(err)
  }

  if len(os.Args) < 2 {
		color.Red.Println("expected a command. Open help to view commands.")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "help", "h":
		fmt.Printf(`floraad is a Source Code Manager; a git alternative.

User and Project commands:

  ru      This command registers a user. It expects the full name of the user and the email.
          Example: floraad ru 'Bankole Ojo' ojobankole@gmail.com

  cp      It expects the name of the project, a Google Cloud Storage bucket name and a service account key file. 
          Create a Google Cloud Storage Bucket at https://console.cloud.google.com with fine-grained permissions.
          Read https://cloud.google.com/docs/authentication/production on how to create a service account key file.
          Copy the Service Account Key file to '%s'. Provide only the name of the 
          Service Account key file without the full path.
          Example: floraad cp myproject myproject myproject-d29134.json




Snapshot Commands:
\n`, rootPath)
	case "ru":
		if len(os.Args) != 4 {
			color.Red.Println("'ru' command expects a name and an email.")
			os.Exit(1)			
		}

		userData := map[string]string {
  		"fullname": os.Args[2],
  		"email": os.Args[3],
  	}

  	jsonBytes, err := json.Marshal(userData)
  	if err != nil {
  		printError(errors.Wrap(err, "json error"))
			os.Exit(1)			
  	}

  	err = os.WriteFile(filepath.Join(rootPath, "user_data.json"), jsonBytes, 0777)
  	if err != nil {
  		printError(errors.Wrap(err, "os write error"))
			os.Exit(1)			
  	}


	case "cp":
		if len(os.Args) != 5 {
			color.Red.Println("'cp' command expects a project name, a GCP bucket name and a service account key file.")
			os.Exit(1)
		}
		
		userData, err := getUserData()
		if err != nil {
			printError(err)
			os.Exit(1)
		}
		projectName := os.Args[2]
		projectData := map[string]string {
			"project_name": os.Args[2],
			"gcp_bucket": os.Args[3],
			"sak_json": os.Args[4],
		}

		outPath := filepath.Join(rootPath, "pd", projectName + ".zconf")
		err = os.WriteFile(outPath, []byte(projectTemplate), 0777)
		if err != nil {
			printError(errors.Wrap(err, "os error"))
			os.Exit(1)
		}

		jsonBytes, err := json.Marshal(projectData)
		if err != nil {
			printError(errors.Wrap(err, "json error"))
			os.Exit(1)
		}

		err = os.WriteFile(filepath.Join(rootPath, "pd", projectData["project_name"] + ".json"), jsonBytes, 0777)
		if err != nil {
			printError(errors.Wrap(err, "os write error"))
			os.Exit(1)
		}

		jsonBytes2, err := json.Marshal(userData)
		err = uploadFile(projectData["gcp_bucket"], sakPath, "users/" + userData["email"], jsonBytes2)
		if err != nil {
			printError(err)
			os.Exit(1)
		}

	default:
		color.Red.Println("Unexpected command. Run the cli with --help to find out the supported commands.")
		os.Exit(1)
	}

}