package config

import (
	"encoding/json"
	"io/ioutil"
)

/*
{
	[
	{
	"application":"live",
	"live":"on",
	"hls":"on",
	"static_push":["rtmp://xx/live"]
	}
	]
}
*/
type Application struct {
	Appname     string
	Liveon      string
	Hlson       string
	Static_push []string
}

type ServerCfg struct {
	Server []Application
}

var RtmpServercfg = ServerCfg{
	Server: []Application{{Appname: "live", Liveon: "on"}},
}

func LoadConfig(configfilename string) error {
	data, err := ioutil.ReadFile(configfilename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &RtmpServercfg)
	if err != nil {
		return err
	}
	return nil
}

func CheckAppName(appname string) bool {
	for _, app := range RtmpServercfg.Server {
		if (app.Appname == appname) && (app.Liveon == "on") {
			return true
		}
	}
	return false
}

func GetStaticPushUrlList(appname string) ([]string, bool) {
	for _, app := range RtmpServercfg.Server {
		if (app.Appname == appname) && (app.Liveon == "on") {
			if len(app.Static_push) > 0 {
				return app.Static_push, true
			} else {
				return nil, false
			}
		}

	}
	return nil, false
}
