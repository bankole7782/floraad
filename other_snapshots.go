package main

import (
	"net/http"
	"github.com/gorilla/mux"
	"path/filepath"
	"github.com/pkg/errors"
	"html/template"
	"time"
	"encoding/json"
	"strings"
  "cloud.google.com/go/storage"
	"context"
  "google.golang.org/api/option"
  "google.golang.org/api/iterator"
)



func viewOthersSnapshots(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	otherEmail := vars["email"]
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

	manifestStatus, err := doesGCPPathExists(pd["project_name"], sakPath, otherEmail + "/manifest.json")
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
	if manifestStatus {
		manifestRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, otherEmail + "/manifest.json")
		if err != nil {
			errorPage(w, err)
			return
		}

		err = json.Unmarshal(manifestRaw, &snapshots)
		if err != nil {
			errorPage(w, errors.Wrap(err, "json error"))
			return
		}
	}

	ctx := context.Background()
  client, err := storage.NewClient(ctx, option.WithCredentialsFile(sakPath))
  if err != nil {
    errorPage(w, errors.Wrap(err, "storage error"))
    return
  }
  defer client.Close()

  users := make([]string, 0)
  it := client.Bucket(pd["gcp_bucket"]).Objects(ctx, &storage.Query{Prefix: "users/"})
  for {
    attrs, err := it.Next()
    if err == iterator.Done {
      break
    }
    if err != nil {
      errorPage(w, errors.Wrap(err, "storage error"))
    	return
    }
    if attrs.Name != "users/" {
    	s := strings.ReplaceAll(attrs.Name, "users/", "")
    	if s == userData["email"] {
    		continue
    	}
    	users = append(users, s)
    }
  }

	otherUserDataRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, "users/" + otherEmail)
	if err != nil {
		errorPage(w, err)
		return
	}
	otherUserData := make(map[string]string)
	err = json.Unmarshal(otherUserDataRaw, &otherUserData)
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
		Users []string
		OtherName string
		OtherEmail string
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
		newS = strings.ReplaceAll(s, "\n", "<br>")
		return template.HTML(newS)
	}

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/view_others_snapshots.html"))
  tmpl.Execute(w, Context{projects, projectName, snapshots, st, csd, users, 
  	otherUserData["fullname"], otherEmail})
}