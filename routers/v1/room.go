package routerV1

import (
	controllersV1 "github.com/ShiinaAiiko/meow-whisper-core/controllers/v1"
	"github.com/ShiinaAiiko/meow-whisper-core/services/middleware"
)

func (r Routerv1) InitRoom() {
	//  /encryption/rsapublickey
	//  /encryption/rsakey
	c := new(controllersV1.RoomController)

	role := middleware.RoleMiddlewareOptions{
		BaseUrl: r.BaseUrl,
	}
	r.Group.POST(role.SetRole(apiUrl["createRoom"], &middleware.RoleOptionsType{
		Authorize:          true,
		RequestEncryption:  true,
		ResponseEncryption: true,
		ResponseDataType:   "protobuf",
	}), c.CreateRoom)

}
