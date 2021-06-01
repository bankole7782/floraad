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
	"io/fs"
	"fmt"
	"bytes"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/span"
  "github.com/hexops/gotextdiff/myers"
  "github.com/otiai10/copy"
)


type ExRules struct {
	Dirs []string
	Extensions []string
	Files []string
}


func getExclusionRules(projectName string) (ExRules, error) {
	rootPath, _ := GetRootPath()
	exRulesPath := filepath.Join(rootPath, "pd", projectName + "_exrules.txt")
	if ! DoesPathExists(exRulesPath) {
		return ExRules{}, nil
	}

	raw, err := os.ReadFile(exRulesPath)
	if err != nil {
		return ExRules{}, errors.Wrap(err, "os error")
	}
	dirs := make([]string, 0)
	extensions := make([]string, 0)
	files := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "/") {
			dirs = append(dirs, line[: len(line) - 1])
		} else if strings.HasPrefix(line, ".") {
			extensions = append(extensions, line)
		} else {
			files = append(files, line)
		}
	}
	return ExRules{dirs, extensions, files}, nil
}


func getCleanFilesList(projectName string) ([]string, error) {
	exRules, err := getExclusionRules(projectName)
	if err != nil {
		return nil, err
	}
	rootPath, _ := GetRootPath()
	projectPath := filepath.Join(rootPath, "p", projectName)

	retFiles := make([]string, 0)

	err = filepath.Walk(projectPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ! info.IsDir() {
			pathToWrite := strings.Replace(path, projectPath + "/", "", 1)
			dirStatus := checkExrulesDir(pathToWrite, exRules)
			extStatus := checkExrulesExtensions(pathToWrite, exRules)
			fileStatus := checkExrulesFiles(pathToWrite, exRules)

			if dirStatus == true && extStatus == true && fileStatus == true {
				retFiles = append(retFiles, filepath.Join(projectPath, pathToWrite))
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "filepath error")
	}
	return retFiles, nil
}


func getAllFilesList(inPath string) ([]string, error) {
	retFiles := make([]string, 0)

	err := filepath.Walk(inPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ! info.IsDir() {
			pathToWrite := strings.Replace(path, inPath + "/", "", 1)
			retFiles = append(retFiles, filepath.Join(inPath, pathToWrite))
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "filepath error")
	}
	return retFiles, nil
}


func checkExrulesDir(path string, exrules ExRules) bool {
	for _, dir := range exrules.Dirs {
		if strings.HasPrefix(path, dir) {
			return false
		}
	}
	return true
}


func checkExrulesExtensions(path string, exRules ExRules) bool {
	for _, ext := range exRules.Extensions {
		if strings.HasSuffix(path, ext) {
			return false
		}
	}
	return true
}


func checkExrulesFiles(path string, exRules ExRules) bool {
	for _, filee := range exRules.Files {
		if filee == path {
			return false
		}
	}
	return true
}


func makeHTMLFriendly(s string) string {
	s = strings.ReplaceAll(s, "/", "__")
	return s
}


func createSnapshot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()
	projectPath := filepath.Join(rootPath, "p", projectName)

	// check if the directory is empty
	objFIs, err := os.ReadDir(projectPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}

	if len(objFIs) == 0 {
		errorPage(w, errors.New("Your project folder is empty."))
		return
	}

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
	manifestStatus, err := doesGCPPathExists(pd["project_name"], sakPath, userData["email"] + "/manifest.json")
	if err != nil {
		errorPage(w, err)
		return
	}

	manifestObj := make([]map[string]string, 0)
	if manifestStatus {
		manifestRaw, err := downloadFileAsBytes(pd["gcp_bucket"], sakPath, userData["email"] + "/manifest.json")
		if err != nil {
			errorPage(w, err)
			return
		}
		err = json.Unmarshal(manifestRaw, &manifestObj)
		if err != nil {
			errorPage(w, errors.Wrap(err, "json error"))
			return
		}
	}



	if r.Method == http.MethodGet {
		if manifestStatus {
			lastSnapshotName := manifestObj[0]["snapshot_name"]
			// get the last snapshot for comparison
			lastSnapshotTar, err := downloadFileAsBytes(pd["gcp_bucket"], sakPath, userData["email"]  + "/" + lastSnapshotName + ".tar.gz")
			if err != nil {
				errorPage(w, err)
				return
			}

			lastSnapshotPath := filepath.Join(rootPath, "flotmp", projectName, lastSnapshotName + ".tar.gz")
			os.MkdirAll(filepath.Join(rootPath, "flotmp", projectName), 0777)
			err = os.WriteFile(lastSnapshotPath, lastSnapshotTar, 0777)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}
			lastSnapshotUndoPath := filepath.Join(rootPath, "flotmp", projectName, lastSnapshotName)
			os.RemoveAll(lastSnapshotUndoPath)
			err = archiver.Unarchive(lastSnapshotPath, lastSnapshotUndoPath)
			if err != nil {
				errorPage(w, errors.Wrap(err, "archiver error"))
				return
			}

			objsList, err := getCleanFilesList(projectName)
			if err != nil {
				errorPage(w, err)
				return
			}

			added := make(map[string]string)
			deleted := make(map[string]string)
			changed := make(map[string]string)
			for _, path := range objsList {
				shortPath := strings.Replace(path, projectPath + "/", "", 1)
				if DoesPathExists(filepath.Join(lastSnapshotUndoPath, shortPath)) {
					// do a deep compare
					rawNew, err := os.ReadFile(path)
					if err != nil {
						errorPage(w, errors.Wrap(err, "os error"))
						return
					}
					rawOld, err := os.ReadFile(filepath.Join(lastSnapshotUndoPath, shortPath))
					if err != nil {
						errorPage(w, errors.Wrap(err, "os error"))
						return
					}

					if ! bytes.Equal(rawNew, rawOld) {
						changed[shortPath] = makeHTMLFriendly(shortPath)
					}
				} else {
					added[shortPath] = path
				}

			}

			oldObjList, err := getAllFilesList(lastSnapshotUndoPath)
			if err != nil {
				errorPage(w, err)
				return
			}
			for _, oldObjPath := range oldObjList {
				shortPath := strings.Replace(oldObjPath, lastSnapshotUndoPath + "/", "", 1)
				if ! DoesPathExists(filepath.Join(projectPath, shortPath)) {
					deleted[shortPath] = oldObjPath
				}
			}

			// compute diffs 
			diffs := make(map[string]template.HTML)
			for key, _ := range changed {
				rawNew, err := os.ReadFile(filepath.Join(projectPath, key))
				if err != nil {
					errorPage(w, err)
					return
				}

				rawOld, err := os.ReadFile(filepath.Join(lastSnapshotUndoPath, key))
				if err != nil {
					errorPage(w, err)
					return
				}

				edits := myers.ComputeEdits(span.URIFromPath(filepath.Base(key)), string(rawOld), string(rawNew))
				diff := fmt.Sprint(gotextdiff.ToUnified("old", "new", string(rawOld), edits))
				diff = strings.ReplaceAll(diff, "\n", "<br>")
				diffs[makeHTMLFriendly(key)] = template.HTML(diff)
			}

			if len(added) == 0 && len(changed) == 0 && len(deleted) == 0 {
				errorPage(w, errors.New("No changes made."))
				return
			}
			type Context struct {
				CurrentProject string
				HasMoreInfo bool
				Added map[string]string
				Changed map[string]string
				Deleted map[string]string
				Diffs map[string]template.HTML
			}

			tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/create_snapshot.html"))
		  tmpl.Execute(w, Context{projectName, true, added, changed, deleted, diffs})					

		} else {

			type Context struct {
				CurrentProject string
				HasMoreInfo bool
			}
			tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/create_snapshot.html"))
		  tmpl.Execute(w, Context{projectName, false})					

		}

	} else {


		if ! manifestStatus {
			// make the first commit
			outObjs, err := getCleanFilesList(projectName)
			if err != nil {
				errorPage(w, err)
				return
			}

			tmpPath := filepath.Join(rootPath, "flotmp", UntestedRandomString(10))
			err = os.MkdirAll(tmpPath, 0777)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}
			for _, p := range outObjs {
				newP := strings.Replace(p, projectPath + "/", "", 1)
				copy.Copy(p, filepath.Join(tmpPath, newP))
			}

			objFIs, err := os.ReadDir(tmpPath)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}

			toArchivePaths := make([]string, 0)
			for _, objFI := range objFIs {
				toArchivePaths = append(toArchivePaths, filepath.Join(tmpPath, objFI.Name()))
			}

		  snapshotName := time.Now().Format(VersionFormat)
		  outFilePath := filepath.Join(rootPath, "flotmp", snapshotName + ".tar.gz")
		  err = archiver.Archive(toArchivePaths, outFilePath)
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

			outObjs, err := getCleanFilesList(projectName)
			if err != nil {
				errorPage(w, err)
				return
			}

			tmpPath := filepath.Join(rootPath, "flotmp", UntestedRandomString(10))
			err = os.MkdirAll(tmpPath, 0777)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}
			for _, p := range outObjs {
				newP := strings.Replace(p, projectPath + "/", "", 1)
				copy.Copy(p, filepath.Join(tmpPath, newP))
			}

			objFIs, err := os.ReadDir(tmpPath)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}

			toArchivePaths := make([]string, 0)
			for _, objFI := range objFIs {
				toArchivePaths = append(toArchivePaths, filepath.Join(tmpPath, objFI.Name()))
			}

		  snapshotName := time.Now().Format(VersionFormat)
		  outFilePath := filepath.Join(rootPath, "flotmp", snapshotName + ".tar.gz")
		  err = archiver.Archive(toArchivePaths, outFilePath)
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

			aManifestObj := map[string]string {
	  		"snapshot_name": snapshotName,
	  		"snapshot_desc": r.FormValue("desc"),
	  	}

	  	newManifestObj := append([]map[string]string{aManifestObj}, manifestObj...)

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