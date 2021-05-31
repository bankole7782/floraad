package main

import (
  "fmt"
  "html/template"
  "strings"
  "net/http"
  "os"
  "path/filepath"
  "github.com/pkg/errors"
  "math/rand"
  "time"
  "encoding/json"
	"cloud.google.com/go/storage"
	"context"
  "google.golang.org/api/option"
)

const VersionFormat = "20060102T150405MST"

type WordPosition struct {
  Word string
  ParagraphIndex int
  HtmlFilename string
}


func GetRootPath() (string, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "os error")
	}
	dd := os.Getenv("SNAP_USER_COMMON")
	if strings.HasPrefix(dd, filepath.Join(hd, "snap", "go")) || dd == "" {
		dd = filepath.Join(hd, "floraad_data")
    os.MkdirAll(dd, 0777)
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


func errorPage(w http.ResponseWriter, err error) {
	type Context struct {
		Msg template.HTML
	}
	msg := fmt.Sprintf("%+v", err)
	fmt.Println(msg)
	msg = strings.ReplaceAll(msg, "\n", "<br>")
	msg = strings.ReplaceAll(msg, " ", "&nbsp;")
	msg = strings.ReplaceAll(msg, "\t", "&nbsp;&nbsp;")
	tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/error.html"))
	tmpl.Execute(w, Context{template.HTML(msg)})
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

  ctx, cancel := context.WithTimeout(ctx, time.Second*50)
  defer cancel()

  // Upload an object with storage.Writer.
  wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
  wc.Write(objectData)
  if err := wc.Close(); err != nil {
    return errors.Wrap(err, "storage error")
  }
  return nil
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