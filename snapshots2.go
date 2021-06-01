package main

import(
	"net/http"
	"github.com/gorilla/mux"
	"path/filepath"
	"encoding/json"
	"github.com/pkg/errors"
	archiver "github.com/mholt/archiver/v3"
	"strings"
	"time"
	"os"
	"html/template"
  "github.com/otiai10/copy"
)


func viewSnapshot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	snapshotName := vars["sname"]
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

	var snapshotDesc string
	for _, snapshotObj := range snapshots {
		if snapshotObj["snapshot_name"] == snapshotName {
			snapshotDesc = snapshotObj["snapshot_desc"]
		}
	}

	snapshotRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/" + snapshotName + ".tar.gz")
	if err != nil {
		errorPage(w, err)
		return
	}
	snapshotPath := filepath.Join(rootPath, "flotmp", projectName, snapshotName + ".tar.gz")
	os.MkdirAll(filepath.Join(rootPath, "flotmp", projectName), 0777)
	err = os.WriteFile(snapshotPath, snapshotRaw, 0777)	
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}
	snapshotUndoPath := filepath.Join(rootPath, "flotmp", projectName, snapshotName)
	os.RemoveAll(snapshotUndoPath)
	err = archiver.Unarchive(snapshotPath, snapshotUndoPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "archiver error"))
		return
	}

	filesInSnapshot := make(map[string]string)
	oldObjList, err := getAllFilesList(snapshotUndoPath)
	if err != nil {
		errorPage(w, err)
		return
	}
	for _, oldObjPath := range oldObjList {
		shortPath := strings.Replace(oldObjPath, snapshotUndoPath + "/", "", 1)
		filesInSnapshot[shortPath] = oldObjPath
	}

	type Context struct {
		Projects []string
		CurrentProject string
		SnapshotName string
		SnapshotTime string
		SnapshotDesc template.HTML
		FilesInSnapshot map[string]string
		SnapshotPath string
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

	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/view_snapshot.html"))
  tmpl.Execute(w, Context{projects, projectName, snapshotName, st(snapshotName), csd(snapshotDesc),
  	filesInSnapshot, snapshotUndoPath})
}


func revertToThis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	snapshotName := vars["sname"]
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

	// download and replace path
	snapshotRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/" + snapshotName + ".tar.gz")
	if err != nil {
		errorPage(w, err)
		return
	}
	snapshotPath := filepath.Join(rootPath, "flotmp", projectName, snapshotName + ".tar.gz")
	os.MkdirAll(filepath.Join(rootPath, "flotmp", projectName), 0777)
	err = os.WriteFile(snapshotPath, snapshotRaw, 0777)	
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}
	snapshotUndoPath := filepath.Join(rootPath, "flotmp", projectName, snapshotName)
	err = archiver.Unarchive(snapshotPath, snapshotUndoPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "archiver error"))
		return
	}

	snapshotObjFIs, err := os.ReadDir(snapshotUndoPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}

	projectPath := filepath.Join(rootPath, "p", projectName)
	emptyDir(projectPath)
	for _, snapshotObjFI := range snapshotObjFIs {
		copy.Copy(filepath.Join(snapshotUndoPath, snapshotObjFI.Name()), filepath.Join(projectPath, snapshotObjFI.Name()))
	}

	// upload snapshot object
	newSnapshotName := time.Now().Format(VersionFormat)

  err = uploadFile(pd["gcp_bucket"], sakPath, userData["email"] + "/" + newSnapshotName + ".tar.gz", snapshotRaw)
  if err != nil {
  	errorPage(w, errors.Wrap(err, "storage error"))
  	return
  }		  


	// update manifest
	manifestRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/manifest.json")
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

	var snapshotDesc string
	for _, snapshotObj := range snapshots {
		if snapshotObj["snapshot_name"] == snapshotName {
			snapshotDesc = snapshotObj["snapshot_desc"]
		}
	}

	aManifestObj := map[string]string {
		"snapshot_name": newSnapshotName,
		"snapshot_desc": snapshotDesc + "\n\nThis snapshot was created after a revert action",
	}
	newManifestObj := append([]map[string]string{aManifestObj}, snapshots...)

	jsonBytes, err := json.Marshal(newManifestObj)
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

}