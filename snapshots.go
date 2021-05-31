package main

import (
	"net/http"
	"github.com/gorilla/mux"
	"path/filepath"
	"os"
	"github.com/pkg/errors"
	"html/template"
	archiver "github.com/mholt/archiver/v3"
	"time"
	"encoding/json"
	"strings"
)


func createSnapshot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()

	pd, err := getProjectData(projectName)
	if err != nil {
		errorPage(w, err)
		return
	}
	userData, err := getUserData()
	if err != nil {
		errorPage(w, err)
		return
	}

	sakPath := filepath.Join(rootPath, pd["sak_json"])
	manifestStatus, err := doesGCPPathExists(pd["project_name"], sakPath, userData["email"] + "/" + "manifest.json")
	if err != nil {
		errorPage(w, err)
		return
	}

	projectPath := filepath.Join(rootPath, "p", projectName)
	if ! manifestStatus {
		// make the first snapshot
		objFIs, err := os.ReadDir(projectPath)
		if err != nil {
			errorPage(w, errors.Wrap(err, "os error"))
			return
		}

		if len(objFIs) == 0 {
			errorPage(w, errors.New("Your project folder is empty."))
			return
		}
	}

	if r.Method == http.MethodGet {
		type Context struct {
			CurrentProject string
		}
		tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/create_snapshot.html"))
	  tmpl.Execute(w, Context{projectName})		
	} else {


		if ! manifestStatus {
		  outFIs, err := os.ReadDir(projectPath)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "os error"))
		  	return
		  }

		  outObjs := make([]string, 0)
		  for _, outFI := range outFIs {
		    outObjs = append(outObjs, filepath.Join(projectPath, outFI.Name()))
		  }

		  snapshotName := time.Now().Format(VersionFormat)
		  outFilePath := filepath.Join(rootPath, "flotmp", snapshotName + ".tar.gz")
		  err = archiver.Archive(outObjs, outFilePath)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "archiver error"))
		  	return
		  }

		  raw, err := os.ReadFile(outFilePath)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "os error"))
		  	return
		  }

		  err = uploadFile(pd["gcp_bucket"], sakPath, userData["email"] + "/" + snapshotName + ".tar.gz", raw)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "storage error"))
		  	return
		  }		  

		  manifestObj := []map[string]string {
		  	{
		  		"snapshot_name": snapshotName,
		  		"snapshot_desc": r.FormValue("desc"),
		  	},
		  }

		  jsonBytes, err := json.Marshal(manifestObj)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "json error"))
		  	return
		  }
		  err = uploadFile(pd["gcp_bucket"], sakPath, userData["email"] + "/manifest.json", jsonBytes)
		  if err != nil {
		  	errorPage(w, errors.Wrap(err, "storage error"))
		  	return
		  }

		  http.Redirect(w, r, "/view_snapshots/" + projectName, 307)		  
		} else {

		}


	}

}


func viewSnapshots(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()

	pd, err := getProjectData(projectName)
	if err != nil {
		errorPage(w, err)
		return
	}
	userData, err := getUserData()
	if err != nil {
		errorPage(w, err)
		return
	}
	sakPath := filepath.Join(rootPath, pd["sak_json"])

	manifestRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/manifest.json")
	if err != nil {
		errorPage(w, err)
		return
	}
	projects, err := getAllProjects()
	if err != nil {
		errorPage(w, err)
		return
	}

	snapshots := make([]map[string]string, 0)
	err = json.Unmarshal(manifestRaw, &snapshots)
	if err != nil {
		errorPage(w, errors.Wrap(err, "json error"))
		return
	}

	type Context struct {
		Projects []string
		CurrentProject string
		Snapshots []map[string]string
		SnapshotTime func(s string) string
		CleanSnapshotDesc func(s string) template.HTML
	}

	st := func(s string) string {
		timeParsed, err :=  time.Parse(VersionFormat, s)
		if err != nil {
			return ""
		}
		return timeParsed.String()
	}

	csd := func(s string) template.HTML {
		newS := strings.ReplaceAll(s, "\r\n", "<br>")
		return template.HTML(newS)
	}

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/view_snapshots.html"))
  tmpl.Execute(w, Context{projects, projectName, snapshots, st, csd})
}