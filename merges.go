package main

import (
	"net/http"
	"github.com/gorilla/mux"
	"path/filepath"
  "github.com/otiai10/copy"
  "encoding/json"
  "github.com/pkg/errors"
  "os"
	archiver "github.com/mholt/archiver/v3"
	"strings"
	"bytes"
	"time"
	"fmt"
	"io/fs"

)


func startMerge(w http.ResponseWriter, r *http.Request) {
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

	// download and unpack the lastest snapshot of other
	latestOtherSnapshots := make([]map[string]string, 0)
	manifestRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, otherEmail + "/manifest.json")
	if err != nil {
		errorPage(w, err)
		return
	}

	err = json.Unmarshal(manifestRaw, &latestOtherSnapshots)
	if err != nil {
		errorPage(w, errors.Wrap(err, "json error"))
		return
	}

	latestOtherSnapshotName := latestOtherSnapshots[0]["snapshot_name"]

	latestOtherSnapshotRaw, err := downloadFileAsBytes(pd["project_name"], sakPath, otherEmail + "/" + latestOtherSnapshotName + ".tar.gz")
	if err != nil {
		errorPage(w, err)
		return
	}
	latestOtherSnapshotPath := filepath.Join(rootPath, "flotmp", projectName, latestOtherSnapshotName + ".tar.gz")
	os.MkdirAll(filepath.Join(rootPath, "flotmp", projectName), 0777)
	err = os.WriteFile(latestOtherSnapshotPath, latestOtherSnapshotRaw, 0777)	
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}
	latestOtherSnapshotUndoPath := filepath.Join(rootPath, "flotmp", projectName, latestOtherSnapshotName)
	os.RemoveAll(latestOtherSnapshotUndoPath)
	err = archiver.Unarchive(latestOtherSnapshotPath, latestOtherSnapshotUndoPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "archiver error"))
		return
	}


	// download and unpack the lastest snapshot of the user
	snapshots := make([]map[string]string, 0)
	manifestRaw, err = downloadFileAsBytes(pd["project_name"], sakPath, userData["email"] + "/manifest.json")
	if err != nil {
		errorPage(w, err)
		return
	}

	err = json.Unmarshal(manifestRaw, &snapshots)
	if err != nil {
		errorPage(w, errors.Wrap(err, "json error"))
		return
	}

	snapshotName := snapshots[0]["snapshot_name"]

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

	// finished preparations. starting the merging.
	otherFileList, err := getAllFilesList(latestOtherSnapshotUndoPath)
	if err != nil {
		errorPage(w, err)
		return
	}
	currentUserFileList, err := getAllFilesList(snapshotUndoPath)
	if err != nil {
		errorPage(w, err)
		return
	}

	finalPath := filepath.Join(rootPath, "p", projectName, "merging_final")
	fromYoursPath := filepath.Join(rootPath, "p", projectName, "merging_yours")
	fromOtherPath := filepath.Join(rootPath, "p", projectName, "merging_other")

	os.MkdirAll(finalPath, 0777)
	os.MkdirAll(fromYoursPath, 0777)
	os.MkdirAll(fromOtherPath, 0777)


	for _, path := range currentUserFileList {
		shortPath := strings.ReplaceAll(path, snapshotUndoPath + "/", "")
		if DoesPathExists(filepath.Join(latestOtherSnapshotUndoPath, shortPath)) {
			// do a deep compare
			rawNew, err := os.ReadFile(path)
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}
			rawOld, err := os.ReadFile(filepath.Join(latestOtherSnapshotUndoPath, shortPath))
			if err != nil {
				errorPage(w, errors.Wrap(err, "os error"))
				return
			}

			if bytes.Equal(rawNew, rawOld) {
				copy.Copy(path, filepath.Join(finalPath, shortPath))
			} else {
				copy.Copy(path, filepath.Join(fromYoursPath, shortPath))
				copy.Copy(filepath.Join(latestOtherSnapshotUndoPath, shortPath),
					filepath.Join(fromOtherPath, shortPath))
			}
		} else {
			copy.Copy(path, filepath.Join(finalPath, shortPath))
		}
	}

	for _, path := range otherFileList {
		shortPath := strings.ReplaceAll(path, latestOtherSnapshotUndoPath + "/", "")
		if ! DoesPathExists(filepath.Join(snapshotUndoPath, shortPath)) {
			copy.Copy(path, filepath.Join(finalPath, shortPath))
		}
	}

	err = os.WriteFile(filepath.Join(rootPath, "p", projectName, ".merging_details.txt"),
		[]byte(otherEmail + "\n" + latestOtherSnapshotName), 0777)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}

	http.Redirect(w, r, "/view_project/" + projectName, 307)
}



func cancelMerge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()

	os.RemoveAll(filepath.Join(rootPath, "p", projectName, ".merging_details.txt"))
	os.RemoveAll(filepath.Join(rootPath, "p", projectName, "merging_other"))
	os.RemoveAll(filepath.Join(rootPath, "p", projectName, "merging_yours"))
	os.RemoveAll(filepath.Join(rootPath, "p", projectName, "merging_final"))

	http.Redirect(w, r, "/view_snapshots/" + projectName, 307)
}



func getCleanFilesList2(projectName, inPath string) ([]string, error) {
	exRules, err := getExclusionRules(projectName)
	if err != nil {
		return nil, err
	}

	retFiles := make([]string, 0)

	err = filepath.Walk(inPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if ! info.IsDir() {
			pathToWrite := strings.Replace(path, inPath + "/", "", 1)
			dirStatus := checkExrulesDir(pathToWrite, exRules)
			extStatus := checkExrulesExtensions(pathToWrite, exRules)
			fileStatus := checkExrulesFiles(pathToWrite, exRules)

			if dirStatus == true && extStatus == true && fileStatus == true {
				retFiles = append(retFiles, filepath.Join(inPath, pathToWrite))
			}
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "filepath error")
	}
	return retFiles, nil
}


func completeMerge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectName := vars["proj"]
	rootPath, _ := GetRootPath()
	projectPath := filepath.Join(rootPath, "p", projectName)

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

	finalPath := filepath.Join(rootPath, "p", projectName, "merging_final")
	outObjs, err := getCleanFilesList2(projectName, finalPath)
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
		newP := strings.Replace(p, finalPath + "/", "", 1)
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

  rawMergingDetails, err := os.ReadFile(filepath.Join(rootPath, "p", projectName, ".merging_details.txt"))
  if err != nil {
  	errorPage(w, errors.Wrap(err, "os error"))
  	return
  }
  partsOfMergingDetails := strings.Split(strings.TrimSpace(string(rawMergingDetails)), "\n")

	st := func(s string) string {
		timeParsed, err :=  time.Parse(VersionFormat, s)
		if err != nil {
			return ""
		}
		return timeParsed.String()
	}

	manifestObj := make([]map[string]string, 0)
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

	aManifestObj := map[string]string {
		"snapshot_name": snapshotName,
		"snapshot_desc": fmt.Sprintf("Merger with %s on %s", partsOfMergingDetails[0], st(partsOfMergingDetails[1])),
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

	tmpObjFIs, err := os.ReadDir(tmpPath)
	if err != nil {
		errorPage(w, errors.Wrap(err, "os error"))
		return
	}

	emptyDir(projectPath)
	for _, tmpObjFI := range tmpObjFIs {
		copy.Copy(filepath.Join(tmpPath, tmpObjFI.Name()), filepath.Join(projectPath, tmpObjFI.Name()))
	}

  http.Redirect(w, r, "/view_snapshots/" + projectName, 307)		  	
}
