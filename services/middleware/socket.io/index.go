package socketioMiddleware

import (
	"github.com/ShiinaAiiko/meow-whisper-core/api"
	"github.com/cherrai/nyanyago-utils/nlog"
)

var (
	log              = nlog.New()
	namespace        = api.Namespace[api.ApiVersion]
	routeEventName   = api.EventName[api.ApiVersion]["routeEventName"]
	requestEventName = api.EventName[api.ApiVersion]["requestEventName"]
)
