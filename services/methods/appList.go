package methods

import (
	"fmt"

	conf "github.com/ShiinaAiiko/meow-whisper-core/config"
	dbxV1 "github.com/ShiinaAiiko/meow-whisper-core/dbx/v1"
)

var appIdDbx = dbxV1.AppIdDbx{}

func InitAppList() {

	// ntimer.SetTimeout(func() {
	log.Info("------InitAppList------")
	// log.Info(conf.Config.AppList)

	for i := range conf.Config.AppList {
		// log.Info(v)
		v := &conf.Config.AppList[i]
		fmt.Print(v.Name + " -> ")
		fapp := appIdDbx.GetAppByName(v.Name)
		if fapp == nil {
			capp, err := appIdDbx.CreateApp(v.Name, "")
			if err != nil {
				log.Error(err)
			}
			fmt.Print("appId:" + capp.AppId)
			fmt.Print(" ")
			fmt.Println("appKey:" + capp.AppKey)
			v.AppId = capp.AppId
			v.AppKey = capp.AppKey
		} else {
			fmt.Print("appId:" + fapp.AppId)
			fmt.Print(" ")
			fmt.Println("appKey:" + fapp.AppKey)
			v.AppId = fapp.AppId
			v.AppKey = fapp.AppKey
		}
	}
	// }, 2000)
}

func CheckApp(appId, appKey string) bool {
	for _, v := range conf.Config.AppList {
		if v.AppId == appId && v.AppKey == appKey {
			return true
		}
	}
	return false
}
