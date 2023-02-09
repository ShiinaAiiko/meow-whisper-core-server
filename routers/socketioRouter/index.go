package socketioRouter

import (
	"github.com/ShiinaAiiko/meow-whisper-core/api"
	conf "github.com/ShiinaAiiko/meow-whisper-core/config"
	"github.com/ShiinaAiiko/meow-whisper-core/routers/socketioRouter/v1"
)

var namespace = api.Namespace[api.ApiVersion]
var eventName = api.EventName[api.ApiVersion]

func InitRouter() {
	// fmt.Println(conf.SocketIoServer)

	rv1 := socketioRouter.V1{
		Server: conf.SocketIO,
		Router: socketioRouter.RouterV1{
			Base: namespace["base"],
			Chat: namespace["chat"],
			Room: namespace["room"],
		},
	}
	rv1.Init()
}
