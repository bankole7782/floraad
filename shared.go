package main

import (
  "fmt"
  "strings"
  "os"
  "path/filepath"
  "github.com/pkg/errors"
  "math/rand"
  "time"
  "encoding/json"
	"cloud.google.com/go/storage"
	"context"
  "google.golang.org/api/option"
  "io"
	"github.com/gookit/color"
)

const VersionFormat = "20060102T150405MST"


func GetRootPath() (string, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "os error")
	}
	dd := os.Getenv("SNAP_USER_COMMON")
	if strings.HasPrefix(dd, filepath.Join(hd, "snap", "go")) || dd == "" {

		if os.Getenv("USER_TWO") == "true" {				
			dd = filepath.Join(hd, "floraad_data_two")
	    os.MkdirAll(dd, 0777)
		} else {
			dd = filepath.Join(hd, "floraad_data")
	    os.MkdirAll(dd, 0777)			
		}

	}

	return dd, nil
}


func UntestedRandomString(length int) string {
  var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
  const charset = "abcdefghijklmnopqrstuvwxyz1234567890"

  b := make([]byte, length)
  for i := range b {
    b[i] = charset[seededRand.Intn(len(charset))]
  }
  return string(b)
}


func DoesPathExists(p string) bool {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return false
	}
	return true
}


func printError(err error) {
	msg := fmt.Sprintf("%+v", err)
	color.Red.Println(msg)
}


func emptyDir(path string) error {
	objFIs, err := os.ReadDir(path)
	if err != nil {
		return errors.Wrap(err, "ioutil error")
	}
	for _, objFI := range objFIs {
		os.RemoveAll(filepath.Join(path, objFI.Name()))
	}
	return nil
}


func uploadFile(bucketName, sakPath, objectName string, objectData []byte) error {
  ctx := context.Background()
  client, err := storage.NewClient(ctx, option.WithCredentialsFile(sakPath))
  if err != nil {
    return errors.Wrap(err, "storage error")
  }
  defer client.Close()

  // Upload an object with storage.Writer.
  wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
  wc.Write(objectData)
  if err := wc.Close(); err != nil {
    return errors.Wrap(err, "storage error")
  }
  return nil
}


func downloadFileAsBytes(bucketName, sakPath, objectName string) ([]byte, error) {
  ctx := context.Background()
  client, err := storage.NewClient(ctx, option.WithCredentialsFile(sakPath))
  if err != nil {
    return nil, errors.Wrap(err, "storage error")
  }
  defer client.Close()

  rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
  if err != nil {
  	return nil, errors.Wrap(err, "storage error")
  }
  data, err := io.ReadAll(rc)
  if err != nil {
  	return nil, errors.Wrap(err, "storage error")
  }
  return data, nil
}


func doesGCPPathExists(bucketName, sakPath, objectName string) (bool, error) {
  ctx := context.Background()
  client, err := storage.NewClient(ctx, option.WithCredentialsFile(sakPath))
  if err != nil {
    return false, errors.Wrap(err, "storage error")
  }
  defer client.Close()

  _, err = client.Bucket(bucketName).Object(objectName).NewReader(ctx)
  if err != nil {
  	return false, nil
  }
  return true, nil
}


func getUserData() (map[string]string, error) {
	rootPath, _ := GetRootPath()
	raw, err := os.ReadFile(filepath.Join(rootPath, "user_data.json"))
	if err != nil {
		return nil, errors.Wrap(err, "os error")
	}

	userData := make(map[string]string)
	err = json.Unmarshal(raw, &userData)
	if err != nil {
		return nil, errors.Wrap(err, "json error")
	}

	return userData, nil
}


func getProjectData(projectName string) (map[string]string, error) {
	rootPath, _ := GetRootPath()
	raw, err := os.ReadFile(filepath.Join(rootPath, "pd", projectName + ".json"))
	if err != nil {
		return nil, errors.Wrap(err, "os error")
	}

	projectData := make(map[string]string)
	err = json.Unmarshal(raw, &projectData)
	if err != nil {
		return nil, errors.Wrap(err, "json error")
	}

	return projectData, nil
}


func getAllProjects() ([]string, error) {
	rootPath, _ := GetRootPath()
	objFIs, err := os.ReadDir(filepath.Join(rootPath, "pd"))
	if err != nil {
		return nil, errors.Wrap(err, "os error")
	}
	projects := make([]string, 0)
	for _, objFI := range objFIs {
		if strings.HasSuffix(objFI.Name(), ".json") {
			p := strings.ReplaceAll(objFI.Name(), ".json", "")
			projects = append(projects, p)			
		}
	}
	return projects, nil
}