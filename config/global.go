package conf

import (
	"github.com/ShiinaAiiko/meow-whisper-core-server/services/encryption"

	"github.com/cherrai/nyanyago-utils/nsocketio"
)

var SocketIO *nsocketio.NSocketIO

var EncryptionClient *encryption.EncryptionOption
