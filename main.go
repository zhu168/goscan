package main

import (
	"errors"
	"fmt"
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

//Config Config
type Config struct {
	ListenAndServe      string `yaml:"ListenAndServe"`
	ScannerCMD          string `yaml:"ScannerCMD"`
	BaseAuth            bool   `yaml:"BaseAuth"`
	BaseAuthUser        string `yaml:"BaseAuthUser"`
	BaseAuthPasswd      string `yaml:"BaseAuthPasswd"`
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
	router.GET("/goscan/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>goscan</title>
</head>
<body>
<h1>goscan</h1>
<button id="scanbtn" onclick="scan();"> scan </button>
<button id="downloadbtn" onclick="download();"> download </button>
<a href="/goscan/imgs/" target="imgs">history imgs</a>
<br>
<div style="width:400px;">
<img src="" id="viewimg" style="width:400px;height:auto;display:inline-block;" />
</div>
<input type="hidden" id="filename" value="" />
<script type="text/javascript">
function scan(){
	document.getElementById('scanbtn').disabled='true';
	document.getElementById('scanbtn').innerHTML=" scaning... ";
	var xhr = new XMLHttpRequest();
	xhr.open('GET', '/goscan/scan', true);
	xhr.onreadystatechange = function() {
		if(xhr.readyState == 4 && xhr.status == 200){
			var res = xhr.response;
			var content = 'response' in xhr ? xhr.response : xhr.responseText
			if (content == 'err'){
				alert('err');
				return;
			}else{
				document.getElementById('filename').value=res;
				document.getElementById('viewimg').src='/goscan/viewimg/'+content;
				return;
			}
		}
		document.getElementById('scanbtn').disabled='';
		document.getElementById('scanbtn').innerHTML=" scan ";
	};
	xhr.send();
}
function download(){
	var filename=document.getElementById('filename').value;
	if (filename ==''){
		alert("Can't download,you must be scan before.");
		return;
	}
	location.href="/goscan/downloadimg/"+filename;
}
</script>
</body>
</html>
`)
	})
	router.GET("/goscan/scan", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if ScanimageUse {
			logger.Fatalln("scanner is busy")
			fmt.Fprint(w, `error：scanner is busy,please wait a litter re try.`)
			return
		}
		ScanimageUseLock.Lock()
		ScanimageUse = true
		defer func() {
			ScanimageUse = false
			ScanimageUseLock.Unlock()
		}()
		filename := time.Now().Format("2006-01-02-15-04-05") + ".jpg"
		pathname := CP + "/imgs/" + filename
		command := fmt.Sprintf(GlobelConfig.ScannerCMD, pathname)
		cmd := exec.Command("/bin/bash", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			logger.Fatalf("Execute Shell:%s %sfailed with error:%s", command, output, err.Error())
			fmt.Fprint(w, `error`)
			return
		}
		// logger.Printf("Execute Shell:%s finished with output:\n%s", command, string(output))
		// fmt.Fprint(w, `error`)
		fmt.Fprint(w, filename)
	})
	router.GET("/goscan/viewimg/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		b, err := ioutil.ReadFile(CP + "/imgs/" + filename)
		if err != nil {
			logger.Fatalln(err)
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			logger.Fatalln(err)
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	router.GET("/goscan/downloadimg/:filename", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		filename := p.ByName("filename")
		b, err := ioutil.ReadFile(CP + "/imgs/" + filename)
		if err != nil {
			logger.Fatalln(err)
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
		w.Header().Add("Content-Disposition", "attachment;filename = "+filename)
		_, err = w.Write(b)
		if err != nil {
			logger.Fatalln(err)
			fmt.Fprint(w, `<h1>error</h1>`)
			return
		}
	})
	router.ServeFiles("/goscan/imgs/*filepath", http.Dir(CP+"/imgs"))
	http.ListenAndServe(GlobelConfig.ListenAndServe, router)
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
	result = string(path[0 : i+1])
	return
}

func LoadConfig() (err error) {
	var b []byte
	b, err = ioutil.ReadFile(CP + "/config.yaml")
	if err != nil {
		logger.Fatal(err)
		return
	}
	err = yaml.Unmarshal(b, &GlobelConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	//log.Printf("config:%#v", GlobelConfig)
	return
}
