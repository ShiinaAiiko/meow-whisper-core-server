package routerV1

import (
	controllersV1 "github.com/ShiinaAiiko/meow-whisper-core-server/controllers/v1"
	"github.com/ShiinaAiiko/meow-whisper-core-server/services/middleware"
)

func (r Routerv1) InitUpload() {
	upload := new(controllersV1.UploadController)

	role := middleware.RoleMiddlewareOptions{
		BaseUrl: r.BaseUrl,
	}

	protoOption := middleware.RoleOptionsType{
		Authorize:          true,
		RequestEncryption:  true,
		ResponseEncryption: true,
		ResponseDataType:   "protobuf",
	}

	r.Group.GET(
		role.SetRole("/upload/file", &protoOption),
		upload.UploadFile)
}
