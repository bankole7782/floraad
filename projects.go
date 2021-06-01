package main

import (
	"net/http"
	"html/template"
	"path/filepath"
	"os"
	"github.com/pkg/errors"
	// "strings"
	"encoding/json"
	"github.com/gorilla/mux"
	// "fmt"
	"github.com/gomarkdown/markdown"
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


func joinProject(w http.ResponseWriter, r *http.Request) {
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

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/join_project.html"))
  tmpl.Execute(w, Context{rootPath, projectsPath, files})

}


func endJoinProject(w http.ResponseWriter, r *http.Request) {
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

	jsonBytes2, err := json.Marshal(userData)
	err = uploadFile(projectData["gcp_bucket"], sakPath, "users/" + userData["email"], jsonBytes2)
	if err != nil {
		errorPage(w, err)
		return
	}

	os.MkdirAll(filepath.Join(rootPath, "p", projectData["project_name"]), 0777)

	http.Redirect(w, r, "/view_project/" + projectData["project_name"], 307)
}


func viewProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()

	pd, err := getProjectData(projectName)
	if err != nil {
		errorPage(w, err)
		return
	}
	sakPath := filepath.Join(rootPath, pd["sak_json"])

	descBytes, err := downloadFileAsBytes(pd["project_name"], sakPath, "desc.md")
	if err != nil {
		errorPage(w, err)
		return
	}
	projects, err := getAllProjects()
	if err != nil {
		errorPage(w, err)
		return
	}

	html := markdown.ToHTML(descBytes, nil, nil)
	type Context struct {
		Projects []string
		CurrentProject string
		DescHTML template.HTML
	}

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/view_project.html"))
  tmpl.Execute(w, Context{projects, projectName, template.HTML(html)})
}


func updateDesc(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()

	pd, err := getProjectData(projectName)
	if err != nil {
		errorPage(w, err)
		return
	}
	sakPath := filepath.Join(rootPath, pd["sak_json"])
	descBytes, err := downloadFileAsBytes(pd["project_name"], sakPath, "desc.md")
	if err != nil {
		errorPage(w, err)
		return
	}

	if r.Method == http.MethodGet {
		type Context struct {
			CurrentProject string
			DescMD string
		}
		tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/update_desc.html"))
	  tmpl.Execute(w, Context{projectName, string(descBytes)})		
	} else {
		err = uploadFile(pd["gcp_bucket"], sakPath, "desc.md", []byte(r.FormValue("desc")))
		if err != nil {
			errorPage(w, err)
			return
		}

		http.Redirect(w, r, "/view_project/" + projectName, 307)
	}
}


func updateExclusionRules(w http.ResponseWriter, r *http.Request) {
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

	if r.Method == http.MethodGet {
		rulesStatus, err := doesGCPPathExists(pd["project_name"], sakPath, userData["email"] + "/exrules.txt")
		if err != nil {
			errorPage(w, err)
			return
		}

		var rules string
		if rulesStatus {
			rulesBytes, err := downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/exrules.txt")
			if err != nil {
				errorPage(w, err)
				return
			}
			rules = string(rulesBytes)
		}

		type Context struct {
			CurrentProject string
			Rules string
		}
		tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/update_exrules.html"))
	  tmpl.Execute(w, Context{projectName, rules})		
	} else {

		err = uploadFile(pd["gcp_bucket"], sakPath, userData["email"] + "/exrules.txt", []byte(r.FormValue("exrules")))
		if err != nil {
			errorPage(w, err)
			return
		}

		err = os.WriteFile(filepath.Join(rootPath, "pd", projectName + "_exrules.txt"), []byte(r.FormValue("exrules")), 0777)
		if err != nil {
			errorPage(w, errors.Wrap(err, "os error"))
			return
		}

		http.Redirect(w, r, "/view_project/" + projectName, 307)
	}
}
