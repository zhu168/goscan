package main

import (
	"io/ioutil"
	"os/exec"
	"sync"

	"github.com/kataras/iris/v12"
)

// ScanimageUseLock ScanimageUseLock
var ScanimageUseLock sync.Mutex

// ScanimageUse ScanimageUse
var ScanimageUse = false

func main() {
	app := iris.New()
	app.Get("/", func(ctx iris.Context) {
		ctx.HTML("<h1>Hello World!</h1>")
	})
	app.Get("/goscan/", func(ctx iris.Context) {
		ctx.HTML(`
		<h1>goscan</h1>
		<button onclick="scan();">scan</button>
		<button onclick="location.href='/goscan/downloadimg/'">download</button>
		<br>
		<div style="width:400px;">
		<img src="/goscan/viewimg/" id="viewimg" style="width:400px;height:auto;display:inline-block;" />
		</div>
		<script type="text/javascript">
function scan(){
	var xhr = new XMLHttpRequest();
	xhr.open('GET', '/goscan/scan', false);
	xhr.onreadystatechange = function() {
		if(xhr.readyState == 4 && xhr.status == 200){
			document.getElementById('viewimg').src='/goscan/viewimg/'+'?'+new Date();
		}
	};
	xhr.send();
}
		</script>
		`)

	})
	app.Get("/goscan/scan", func(ctx iris.Context) {
		ScanimageUseLock.Lock()
		if ScanimageUse {
			ScanimageUseLock.Unlock()
			ctx.Application().Logger().Error("scanner is busy")
			ctx.JSON("error：scanner is busy,please wait a litter re try.")
			return
		}
		ScanimageUseLock.Unlock()
		ScanimageUse = true
		defer func() {
			ScanimageUse = false
		}()
		command := `scanimage -d 'hpaio:/usb/HP_LaserJet_M1005?serial=KJ6NYS4' --format jpeg --mode color --resolution 200 > ./scan.jpg`
		cmd := exec.Command("/bin/bash", "-c", command)
		output, err := cmd.Output()
		if err != nil {
			ctx.Application().Logger().Error("Execute Shell:%s failed with error:%s", command, err.Error())
			ctx.JSON("error")
			return
		}
		ctx.Application().Logger().Debug("Execute Shell:%s finished with output:\n%s", command, string(output))
		ctx.JSON("error")
		ctx.JSON("ok")
		return
	})
	app.Get("/goscan/viewimg/", func(ctx iris.Context) {
		// ctx.Application().Logger().Info("viewimg")

		imgname := "scan" //ctx.Params().Get("imgname")

		b, err := ioutil.ReadFile("./" + imgname + ".jpg")
		_, err = ctx.Write(b)
		if err != nil {
			ctx.Application().Logger().Error(err)
			ctx.HTML(`<h1>error</h1>`)
			return
		}
		return
	})
	app.Get("/goscan/downloadimg/", func(ctx iris.Context) {
		ctx.Application().Logger().Info("viewimg")
		imgname := "scan" //ctx.Params().Get("imgname")
		ctx.SendFile("./"+imgname+".jpg", imgname+".jpg")
		return
	})
	app.Listen(":3031")
}
