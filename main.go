package main

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/yaml.v2"
)

//go:embed web/templates
var templatesFS embed.FS

//Config Config
type Config struct {
	ListenAndServe      string `yaml:"ListenAndServe"`
	ScannerCMD          string `yaml:"ScannerCMD"`
	BaseAuth            bool   `yaml:"BaseAuth"`
	BaseAuthUser        string `yaml:"BaseAuthUser"`
	BaseAuthPasswd      string `yaml:"BaseAuthPasswd"`
	BaseAuthSuper       bool   `yaml:"BaseAuthSuper"`
	BaseAuthSuperUser   string `yaml:"BaseAuthSuperUser"`
	BaseAuthSuperPasswd string `yaml:"BaseAuthSuperPasswd"`
}

var GlobelConfig Config
var logger *log.Logger

// ScanimageUseLock ScanimageUseLock
var ScanimageUseLock sync.Mutex

// ScanimageUse ScanimageUse
var ScanimageUse = false
var CP string

func main() {
	var err error
	CP, err = GetCurrentPath()
	if err != nil {
		log.Fatalln("fail to lookPath args 0!")
		return
	}
	err = LoadConfig()
	if err != nil {
		log.Fatalln("fail to LoadConfig!")
		return
	}
	file, err := os.Create(CP + "/goscan.log")
	if err != nil {
		log.Fatalln("fail to create test.log file!")
		return
	}
	logger = log.New(file, "", log.LstdFlags|log.Llongfile)
	router := httprouter.New()
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprint(w, `<h1><a href="/goscan/">goscan</a></h1>`)
	})
	RouterAddGET(router, "user", "/goscan/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		err = Tpl(w, "/goscan.html", nil)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}

	})
	RouterAddGET(router, "user", "/goscan/scan", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if ScanimageUse {
			logger.Printf("scanner is busy")
			fmt.Fprint(w, `error：scanner is busy,please wait a litter re try.`)
			return
		}
		ScanimageUseLock.Lock()
		ScanimageUse = true
		defer func() {
			ScanimageUse = false
			ScanimageUseLock.Unlock()
		}()
		now := time.Now()
		dirname := now.Format("2006-01")
		filename := now.Format("2006-01-02-15-04-05") + ".jpg"
		pathname := CP + "/imgs/" + dirname
		if _, err := os.Stat(pathname); os.IsNotExist(err) {
			err := os.MkdirAll(pathname, 0755)
			if err != nil {
				logger.Printf(err.Error())
				fmt.Fprint(w, `error`)
				return
			}
		}
		pathname = pathname + "/" + filename
		command := fmt.Sprintf(GlobelConfig.ScannerCMD, pathname)
		cmd := exec.Command("/bin/bash", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			logger.Printf("Execute Shell:%s %sfailed with error:%s", command, output, err.Error())
			fmt.Fprint(w, `error`)
			return
		}
		// logger.Printf("Execute Shell:%s finished with output:\n%s", command, string(output))
		// fmt.Fprint(w, `error`)
		fmt.Fprint(w, filename)
	})
	RouterAddGET(router, "user", "/goscan/viewimg/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		err = FileNameCheck(filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		b, err := ioutil.ReadFile(CP + "/imgs/" + filename[:7] + "/" + filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	RouterAddGET(router, "user", "/goscan/downloadimg/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		err = FileNameCheck(filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		b, err := ioutil.ReadFile(CP + "/imgs/" + filename[:7] + "/" + filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		w.Header().Add("Content-Disposition", "attachment;filename = "+filename)
		_, err = w.Write(b)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	RouterAddGET(router, "super", "/goscan/imgs/monthlist", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		tfiles, err := ioutil.ReadDir(CP + "/imgs/")
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		var files []fs.FileInfo
		for i := 0; i < len(tfiles); i++ {
			if tfiles[i].IsDir() {
				files = append(files, tfiles[i])
			}
		}
		tplparms := make(map[string]interface{})
		tplparms["files"] = files
		err = Tpl(w, "/monthlist.html", tplparms)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	RouterAddGET(router, "super", "/goscan/imgs/list/:month", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		month := p.ByName("month")
		err := DirNameCheck(month)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		files, err := ioutil.ReadDir(CP + "/imgs/" + month)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		tplparms := make(map[string]interface{})
		tplparms["files"] = files
		err = Tpl(w, "/imgs.html", tplparms)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	RouterAddGET(router, "super", "/goscan/imgs/view/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		err = FileNameCheck(filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		b, err := ioutil.ReadFile(CP + "/imgs/" + filename[:7] + "/" + filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	RouterAddGET(router, "super", "/goscan/imgs/rm/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		err = FileNameCheck(filename)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `error`)
			return
		}
		fullname := CP + "/imgs/" + filename[:7] + "/" + filename
		if _, err := os.Stat(fullname); os.IsNotExist(err) {
			fmt.Fprint(w, `error`)
			return
		}
		err := os.Remove(fullname)
		if err != nil {
			logger.Printf(err.Error())
			fmt.Fprint(w, `error`)
			return
		}
		fmt.Fprint(w, `ok`)
	})
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, v interface{}) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error:%s", v)
	}
	http.ListenAndServe(GlobelConfig.ListenAndServe, router)
}

func RouterAddGET(r *httprouter.Router, role, path string, handle httprouter.Handle) {
	if GlobelConfig.BaseAuth {
		switch role {
		case "user":
			r.GET(path, UserBasicAuth(handle))
			logger.Printf("add router GET %s UserBasicAuth", path)
		case "super":
			r.GET(path, SuperBasicAuth(handle))
			logger.Printf("add router GET %s SuperBasicAuth", path)
		default:
			logger.Printf("RouterAddGET role name err!")
		}
	} else {
		r.GET(path, handle)
		logger.Printf("add router GET %s", path)
	}
}
func UserBasicAuth(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		user, password, hasAuth := r.BasicAuth()
		if hasAuth && (user == GlobelConfig.BaseAuthUser && password == GlobelConfig.BaseAuthPasswd) {
			h(w, r, ps)
		} else {
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}
func SuperBasicAuth(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		user, password, hasAuth := r.BasicAuth()
		if hasAuth && user == GlobelConfig.BaseAuthSuperUser && password == GlobelConfig.BaseAuthSuperPasswd {
			h(w, r, ps)
		} else {
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

func GetCurrentPath() (result string, err error) {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		i = strings.LastIndex(path, "\\")
	}
	if i < 0 {
		err = errors.New(`error: Can't find "/" or "\"`)
		return
	}
	result = string(path[0:i])
	return
}

func LoadConfig() (err error) {
	var b []byte
	b, err = ioutil.ReadFile(CP + "/config.yaml")
	if err != nil {
		logger.Printf(err.Error())
		return
	}
	err = yaml.Unmarshal(b, &GlobelConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	//log.Printf("config:%#v", GlobelConfig)
	return
}

func FileNameCheck(filename string) (err error) {
	if len(filename) != 23 {
		err = fmt.Errorf("filename err")
		return
	}
	for i, r := range filename {
		if i < 19 {
			if r != 45 && (r < 48 || r > 57) {
				err = fmt.Errorf("filename err")
				return
			}
		}
	}
	if filename[19:] != ".jpg" {
		err = fmt.Errorf("filename err")
		return
	}
	return
}
func DirNameCheck(dirname string) (err error) {
	if len(dirname) != 7 {
		err = fmt.Errorf("dirname err")
		return
	}
	for _, r := range dirname {
		if r != 45 && (r < 48 || r > 57) {
			err = fmt.Errorf("dirname err")
			return
		}
	}
	return
}
func Tpl(w http.ResponseWriter, tplname string, data interface{}) (err error) {
	basedir := "web/templates"
	tpl, err := template.ParseFS(templatesFS, basedir+tplname)
	// tpl, err := template.ParseFiles(basedir + tplname)
	if err != nil {
		return
	}
	err = tpl.Execute(w, data)
	return
}
