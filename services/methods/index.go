package methods

import (
	"github.com/ShiinaAiiko/meow-whisper-core-server/api"
	conf "github.com/ShiinaAiiko/meow-whisper-core-server/config"
	dbxV1 "github.com/ShiinaAiiko/meow-whisper-core-server/dbx/v1"
)

var (
	log         = conf.Log
	contactDbx  = dbxV1.ContactDbx{}
	groupDbx    = dbxV1.GroupDbx{}
	messagesDbx = dbxV1.MessagesDbx{}

	namespaces       = api.Namespace[api.ApiVersion]
	routeEventName   = api.EventName[api.ApiVersion]["routeEventName"]
	requestEventName = api.EventName[api.ApiVersion]["requestEventName"]
)
