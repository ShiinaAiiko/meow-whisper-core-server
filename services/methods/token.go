package methods

import "github.com/ShiinaAiiko/meow-whisper-core-server/protos"

func CreateToken(tokenInfo *protos.MWCToken) (token string, err error) {
	log.Info(tokenInfo)
	return
}
