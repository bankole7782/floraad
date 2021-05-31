package main

import (
	"github.com/webview/webview"
	"github.com/gorilla/mux"
	"net/http"
	"fmt"
	"os"
	"path/filepath"
	"html/template"
	"os/exec"
	"encoding/json"
	"github.com/pkg/errors"
	"strings"
)

var wv webview.WebView

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
	port := "41769"
	debug := false
	if os.Getenv("PANDOLEE_DEVELOPER") == "true" {
		debug = true
	}
  rootPath, err := GetRootPath()
  if err != nil {
    panic(err)
  }

	defer func() {
		emptyDir(filepath.Join(rootPath, "flotns"))
	}()


	go func() {

	  r := mux.NewRouter()

	  r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	  	if ! DoesPathExists(filepath.Join(rootPath, "user_data.json")) {
	      tmpl := template.Must(template.ParseFS(content, "templates/base.html", "templates/save_user_data.html"))
	      tmpl.Execute(w, nil)
	      return
	  	}

	  	dirFIs, err := os.ReadDir(filepath.Join(rootPath, "pd"))
	  	if err != nil {
	  		errorPage(w, errors.Wrap(err, "os read error"))
	  		return
	  	}

	  	if len(dirFIs) == 0 {
	  		http.Redirect(w, r, "/new_project", 307)
	  		return
	  	}
	  	projectName := strings.Replace(dirFIs[0].Name(), ".json", "", 1)
			http.Redirect(w, r, "/view_project/" + projectName, 307)	  	
	  })


	  r.HandleFunc("/save_user_data", func(w http.ResponseWriter, r *http.Request) {
	  	userData := map[string]string {
	  		"fullname": r.FormValue("fullname"),
	  		"email": r.FormValue("email"),
	  	}

	  	jsonBytes, err := json.Marshal(userData)
	  	if err != nil {
	  		errorPage(w, errors.Wrap(err, "json error"))
	  		return
	  	}

	  	err = os.WriteFile(filepath.Join(rootPath, "user_data.json"), jsonBytes, 0777)
	  	if err != nil {
	  		errorPage(w, errors.Wrap(err, "os write error"))
	  		return
	  	}

	  	http.Redirect(w, r, "/", 307)
	  })

		r.HandleFunc("/gs/{obj}", func (w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			rawObj, err := contentStatics.ReadFile("statics/" + vars["obj"])
			if err != nil {
				panic(err)
			}
			w.Header().Set("Content-Disposition", "attachment; filename=" + vars["obj"])
			contentType := http.DetectContentType(rawObj)
			w.Header().Set("Content-Type", contentType)
			w.Write(rawObj)
		})

		r.HandleFunc("/xdg/", func (w http.ResponseWriter, r *http.Request) {
			exec.Command("xdg-open", r.FormValue("p")).Run()
		})

		// projects
		r.HandleFunc("/new_project", newProject)
		r.HandleFunc("/save_project", saveProject)
		r.HandleFunc("/view_project/{proj}", viewProject)
		r.HandleFunc("/update_desc/{proj}", updateDesc)


		// snapshots
		r.HandleFunc("/create_snapshot/{proj}", createSnapshot)
		r.HandleFunc("/view_snapshots/{proj}", viewSnapshots)


	  err := http.ListenAndServe(fmt.Sprintf(":%s", port), r)
	  if err != nil {
	  	panic(err)	
	  }

	}()

	w := webview.New(debug)
	wv = w
	defer w.Destroy()
	w.SetTitle("Floraad: A Source Code Manager.")
	w.SetSize(1200, 800, webview.HintNone)

	w.Navigate(fmt.Sprintf("http://127.0.0.1:%s", port))
	w.Run()

}