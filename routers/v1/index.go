package routerV1

import (
	"github.com/ShiinaAiiko/meow-whisper-core/api"
	"github.com/gin-gonic/gin"
)

type Routerv1 struct {
	Engine  *gin.Engine
	Group   *gin.RouterGroup
	BaseUrl string
}

var apiUrl = api.ApiUrls[api.ApiVersion]

func (r Routerv1) Init() {
	r.Group = r.Engine.Group(r.BaseUrl)
	r.InitEncryption()
	r.InitCall()
	r.InitUpload()
	r.InitUser()
	r.InitSSO()
	r.InitContact()
	r.InitGroup()
	r.IniMessage()
	r.InitFile()
}
