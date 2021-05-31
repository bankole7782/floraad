package main

import (
	"net/http"
	"html/template"
	"path/filepath"
	"os"
	"github.com/pkg/errors"
	// "strings"
	"encoding/json"
)


func newProject(w http.ResponseWriter, r *http.Request) {
	rootPath, _ := GetRootPath()
	projectsPath := filepath.Join(rootPath, "p")

	dirFIs, err := os.ReadDir(rootPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}

	files := make([]string, 0)
	for _, dirFI := range dirFIs {
		if ! dirFI.IsDir() && dirFI.Name() != "user_data.json" {
			files = append(files, dirFI.Name())
		}
	}

	type Context struct {
		RootPath string
		ProjectsPath string
		Files []string
	}

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/new_project.html"))
  tmpl.Execute(w, Context{rootPath, projectsPath, files})

}


func saveProject(w http.ResponseWriter, r *http.Request) {
	rootPath, _ := GetRootPath()

	userData, err := getUserData()
	if err != nil {
		errorPage(w, err)
		return
	}

	projectData := map[string]string {
		"project_name": r.FormValue("project_name"),
		"gcp_bucket": r.FormValue("gcp_bucket"),
		"sak_json": r.FormValue("sak_json"),
	}

	jsonBytes, err := json.Marshal(projectData)
	if err != nil {
		errorPage(w, errors.Wrap(err, "json error"))
		return
	}

	err = os.WriteFile(filepath.Join(rootPath, "pd", projectData["project_name"] + ".json"), jsonBytes, 0777)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os write error"))
		return
	}

	sakPath := filepath.Join(rootPath, projectData["sak_json"])
	err = uploadFile(projectData["gcp_bucket"], sakPath, "desc.md", []byte(r.FormValue("desc")))
	if err != nil {
		errorPage(w, err)
		return
	}

	jsonBytes2, err := json.Marshal(userData)
	err = uploadFile(projectData["gcp_bucket"], sakPath, "users/" + userData["email"], jsonBytes2)
	if err != nil {
		errorPage(w, err)
		return
	}

	os.MkdirAll(filepath.Join(rootPath, "p", projectData["project_name"]), 0777)

	http.Redirect(w, r, "/view_project/" + projectData["project_name"], 307)
}
