package methods

import (
	"errors"
	"fmt"
	"time"

	conf "github.com/ShiinaAiiko/meow-whisper-core/config"
	"github.com/ShiinaAiiko/meow-whisper-core/protos"
	"github.com/ShiinaAiiko/meow-whisper-core/services/response"

	"github.com/cherrai/nyanyago-utils/cipher"
	"github.com/cherrai/nyanyago-utils/nsocketio"
	"github.com/cherrai/nyanyago-utils/nstrings"
	"github.com/cherrai/nyanyago-utils/validation"
	sso "github.com/cherrai/saki-sso-go"
	"github.com/golang-jwt/jwt"
)

func GetAnonymousRoomId(invitationCode string) string {
	return cipher.MD5(("anonymousRoom" + invitationCode))
}

func SendAnonymousRoomMessage(namespace string, eventName string, invitationCode string, data string) {
	SendRoomMessage(namespace, eventName, GetAnonymousRoomId(invitationCode), data)
}

func SendRoomMessage(namespace string, eventName string, roomId string, data string) {
	c := nsocketio.ConnContext{
		ServerContext: conf.SocketIO,
	}
	conns := c.GetAllConnContextInRoom(namespaces[namespace], roomId)

	for _, conn := range conns {
		userInfoInterface := c.GetSessionCacheWithConnId(conn.Conn.ID(), "userInfo")
		if userInfoInterface == nil {
			return
		}
		userInfo := userInfoInterface.(*sso.UserInfo)
		// if userInfo.Uid ==authorId{
		// 	return
		// }

		var res response.ResponseProtobufType
		res.Code = 200
		res.Data = data

		getConnContext := conf.SocketIO.GetConnContextByTag(namespaces["chat"], "Uid", nstrings.ToString(userInfo.Uid))
		for _, c := range getConnContext {
			deviceId := c.GetTag("DeviceId")
			userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, deviceId)
			if userAesKey == nil {
				return
			}
			eventName := routeEventName[eventName]
			responseData := res.Encryption(userAesKey.AESKey, res.GetResponse())
			isEmit := c.Emit(eventName, responseData)
			if isEmit {
				log.Info(namespace, eventName, roomId, "发送成功")
			} else {
				log.Info(namespace, eventName, roomId, "发送失败")
			}
		}
	}
}

func ValidateSecretChatToken(data *protos.SecretChatToken, userAgent *sso.UserAgent) (*sso.UserInfo, error) {
	if data == nil {
		return nil, errors.New("“data.Token”: cannot be blank.")
	}
	err := validation.ValidateStruct(
		data,
		validation.Parameter(&data.SecretChatToken, validation.Required()),
		validation.Parameter(&data.InvitationCode, validation.Required()),
		validation.Parameter(&data.UserToken, validation.Required()),
		validation.Parameter(&data.UserDeviceId, validation.Required()),
	)
	// 1、校验匿名Token
	aes := cipher.AES{
		Key:  conf.Config.SecretChatToken.AesKey,
		Mode: "CFB",
	}
	deToken, err := aes.DecryptWithString(data.SecretChatToken, "")
	if err != nil {
		return nil, err
	}
	tokenData, err := jwt.ParseWithClaims(deToken.String(), &SecretChatTokenClaims{}, func(tokenStr *jwt.Token) (interface{}, error) {
		if _, ok := tokenStr.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", tokenStr.Header["alg"])
		}
		return []byte(conf.Config.SecretChatToken.Key), nil
	})
	if err != nil {
		return nil, err
	}
	if sctInfo, ok := tokenData.Claims.(*SecretChatTokenClaims); ok && tokenData.Valid {
		// log.Info("sctInfo", sctInfo, ok)
		// 2、校验聊天凭证里的邀请码和传入的邀请码是否一致
		if sctInfo.InvitationCode != data.InvitationCode {
			return nil, nil
		}

		// 3、校验聊天凭证Token
		aUser, err := conf.SSO.Verify(data.UserToken, data.UserDeviceId, *userAgent)
		if err != nil {
			return nil, err
		}
		if aUser == nil || aUser.Payload.Uid == 0 {
			return nil, nil
		}

		// 4、校验聊天凭证里的UID和匿名token里的UID是否一致
		if sctInfo.Uid != aUser.Payload.Uid {
			return nil, nil
		}

		// 5、获取用户信息
		return &aUser.Payload, err
	}
	return nil, err
}

type SecretChatTokenClaims struct {
	Uid            int64  `json:"uid"`
	DeviceId       string `json:"deviceId"`
	InvitationCode string `json:"invitationCode"`
	jwt.StandardClaims
}

func GenerateSecretChatToken(uid int64, deviceId, invitationCode string) (string, error) {
	nowTime := time.Now()
	expireTime := nowTime.Add(30 * 24 * time.Hour)

	claims := SecretChatTokenClaims{
		uid,
		deviceId,
		invitationCode,
		jwt.StandardClaims{
			ExpiresAt: expireTime.Unix(),
			Issuer:    conf.Config.SecretChatToken.Issuer,
		},
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tokenClaims.SignedString([]byte(conf.Config.SecretChatToken.Key))
	if err != nil {
		return "", err
	}
	aes := cipher.AES{
		Key:  conf.Config.SecretChatToken.AesKey,
		Mode: "CFB",
	}
	enToken, err := aes.Encrypt(token, "")
	if err != nil {
		return "", err
	}
	return enToken.HexEncodeToString(), err
}
