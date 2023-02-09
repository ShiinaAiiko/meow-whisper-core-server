package controllersV1

import (
	"github.com/ShiinaAiiko/meow-whisper-core/api"
	dbxV1 "github.com/ShiinaAiiko/meow-whisper-core/dbx/v1"
	"github.com/cherrai/nyanyago-utils/nlog"
)

var (
	log            = nlog.New()
	contactDbx     = dbxV1.ContactDbx{}
	groupDbx       = dbxV1.GroupDbx{}
	messagesDbx    = dbxV1.MessagesDbx{}
	namespace      = api.Namespace[api.ApiVersion]
	routeEventName = api.EventName[api.ApiVersion]["routeEventName"]
)
