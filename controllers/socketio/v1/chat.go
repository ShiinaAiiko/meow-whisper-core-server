package socketIoControllersV1

import (
	"errors"
	"time"

	conf "github.com/ShiinaAiiko/meow-whisper-core/config"
	"github.com/ShiinaAiiko/meow-whisper-core/models"
	"github.com/ShiinaAiiko/meow-whisper-core/protos"
	"github.com/ShiinaAiiko/meow-whisper-core/services/methods"
	"github.com/ShiinaAiiko/meow-whisper-core/services/response"
	"github.com/cherrai/nyanyago-utils/cipher"
	"github.com/cherrai/nyanyago-utils/nsocketio"
	"github.com/cherrai/nyanyago-utils/nstrings"
	"github.com/cherrai/nyanyago-utils/validation"
	sso "github.com/cherrai/saki-sso-go"
	"github.com/pion/turn/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// var friendsDbx = new(dbxV1.FriendsDbx)
// var chatDbx = new(dbxV1.ChatDbx)

type ChatController struct {
}

func (cc *ChatController) Connect(e *nsocketio.EventInstance) error {
	log.Info("/Chat => 正在进行连接.")

	var res response.ResponseProtobufType
	c := e.ConnContext()
	baseNS := e.ConnContext().GetConnContext(e.Conn().ID())
	log.Info(baseNS)
	log.Info(baseNS.GetSessionCache("appId"))

	userInfoAny := baseNS.GetSessionCache("userInfo")

	if userInfoAny == nil {
		res.Code = 10004
		response := res.GetResponse()
		baseNS.Emit(routeEventName["error"], response)
		go c.Close()
		return errors.New(res.Error)
	}

	userInfo := userInfoAny.(*sso.AnonymousUserInfo)
	deviceId := baseNS.GetSessionCache("deviceId").(string)
	log.Info("userInfo", userInfo)

	c.SetSessionCache("loginTime", baseNS.GetSessionCache("loginTime"))
	c.SetSessionCache("appId", baseNS.GetSessionCache("appId"))
	c.SetSessionCache("userInfo", userInfo)
	c.SetSessionCache("deviceId", deviceId)
	c.SetSessionCache("userAgent", baseNS.GetSessionCache("userAgent"))
	c.SetTag("Uid", userInfo.Uid)
	c.SetTag("DeviceId", deviceId)

	return nil
}

func (cc *ChatController) Disconnect(e *nsocketio.EventInstance) error {
	log.Info("/Chat => 已经断开了")

	return nil
}
func (cc *ChatController) JoinRoom(e *nsocketio.EventInstance) error {
	// 1、初始化返回体
	var res response.ResponseProtobufType
	log.Info("/Chat => JoinRoom")
	res.Code = 200
	res.CallSocketIo(e)

	// 2、获取参数
	data := new(protos.JoinRoom_Request)
	var err error
	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	// 3、校验参数
	if err = validation.ValidateStruct(
		data,
		validation.Parameter(&data.RoomIds, validation.Required()),
	); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	appId := e.GetSessionCache("appId")
	// deviceId := e.GetSessionCache("deviceId").(string)
	userInfo := e.GetSessionCache("userInfo").(*sso.AnonymousUserInfo)

	log.Info(data, data.RoomIds, appId, userInfo)

	for _, v := range data.RoomIds {
		if b := e.ConnContext().JoinRoomWithNamespace(v); !b {
			res.Errors(err)
			res.Code = 10501
			res.CallSocketIo(e)
			return nil
		}
	}
	res.Code = 200

	responseData := protos.JoinGroup_Response{}
	res.Data = protos.Encode(&responseData)
	res.CallSocketIo(e)

	return nil
}

func (cc *ChatController) SendMessage(e *nsocketio.EventInstance) error {
	// 1、初始化返回体
	var res response.ResponseProtobufType
	log.Info("/Chat => 发送信息")
	res.Code = 200
	res.CallSocketIo(e)

	// 2、获取参数
	data := new(protos.SendMessage_Request)
	var err error
	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	// 3、校验参数
	if err = validation.ValidateStruct(
		data,
		validation.Parameter(&data.Type, validation.Enum([]string{"Group", "Contact"}), validation.Required()),
		validation.Parameter(&data.RoomId, validation.Type("string"), validation.Required()),
		validation.Parameter(&data.AuthorId, validation.Type("string"), validation.Required()),
	); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	if data.Call == nil && data.Image == nil {
		if err = validation.ValidateStruct(
			data.Call,
			validation.Parameter(&data.Message, validation.Type("string"), validation.Required()),
		); err != nil {
			res.Errors(err)
			res.Code = 10002
			res.CallSocketIo(e)
			return err
		}
	}

	if data.Image != nil && data.Image.Url != "" {
		if err = validation.ValidateStruct(
			data.Image,
			validation.Parameter(&data.Image.Url, validation.Required()),
			validation.Parameter(&data.Image.Width, validation.Greater(0), validation.Required()),
			validation.Parameter(&data.Image.Height, validation.Greater(0), validation.Required()),
		); err != nil {
			res.Errors(err)
			res.Code = 10002
			res.CallSocketIo(e)
			return err
		}
	}

	appId := e.GetSessionCache("appId")
	// deviceId := e.GetSessionCache("deviceId").(string)
	userInfo := e.GetSessionCache("userInfo").(*sso.AnonymousUserInfo)

	log.Info(data, appId, userInfo)
	mp := models.Messages{
		RoomId:   data.RoomId,
		AuthorId: data.AuthorId,
		Message:  data.Message,
	}
	if data.ReplyId != "" {
		if replyId, err := primitive.ObjectIDFromHex(data.ReplyId); err == nil {
			mp.ReplyId = replyId
		}
	}

	if data.Call != nil && data.Call.Type != "" {
		p := []*models.MessagesCallParticipants{}
		for _, v := range data.Call.Participants {
			p = append(p, &models.MessagesCallParticipants{
				Uid:    v.Uid,
				Caller: v.Caller,
			})
		}
		mp.Call = &models.MessagesCall{
			Status:       data.Call.Status,
			RoomId:       data.Call.RoomId,
			Participants: p,
			Type:         data.Call.Type,
			Time:         data.Call.Time,
		}
	}

	if data.Image != nil && data.Image.Url != "" {
		mp.Image = &models.MessagesImage{
			Url:    data.Image.Url,
			Width:  data.Image.Width,
			Height: data.Image.Height,
		}
	}

	message, err := messagesDbx.SendMessage(&mp)
	log.Info("message", message, err)
	if err != nil {
		res.Errors(err)
		res.Code = 10401
		res.CallSocketIo(e)
		return err
	}

	// 更新状态
	switch data.Type {
	case "Contact":
		// 更新 Contact
		if err = contactDbx.UpdateContactChatStatus(data.RoomId, message.Id); err != nil {
			res.Errors(err)
			res.Code = 10401
			res.CallSocketIo(e)
			return err
		}
	case "Group":
		// 更新 Group
		if err = groupDbx.UpdateGroupChatStatus(data.RoomId, message.Id); err != nil {
			res.Errors(err)
			res.Code = 10401
			res.CallSocketIo(e)
			return err
		}
		// 更新 GroupMembers
		if err = groupDbx.UpdateGroupMemberChatStatus(data.RoomId, message.Id); err != nil {
			res.Errors(err)
			res.Code = 10401
			res.CallSocketIo(e)
			return err
		}

	}

	res.Code = 200

	fMessage := methods.FormatMessages([]*models.Messages{message})

	if len(fMessage) == 0 {
		res.Errors(err)
		res.Code = 10011
		res.CallSocketIo(e)
		return err
	}

	responseData := protos.SendMessage_Response{
		Message: fMessage[0],
	}
	res.Data = protos.Encode(&responseData)
	// log.Info("res.Data ", res.Data)
	res.CallSocketIo(e)

	msc := methods.SocketConn{
		Conn: e.ConnContext(),
	}

	// 暂定只有1v1需要加密e2ee，其他的用自己的即可
	msc.BroadcastToRoom(data.RoomId,
		routeEventName["receiveMessage"],
		&res,
		false)
	return nil
}

func (cc *ChatController) EditMessage(e *nsocketio.EventInstance) error {
	// 1、初始化返回体
	var res response.ResponseProtobufType
	// log.Info("/Chat => 发送信息")
	res.Code = 200
	res.CallSocketIo(e)

	// 2、获取参数
	data := new(protos.EditMessage_Request)
	var err error
	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	// 3、校验参数
	if err = validation.ValidateStruct(
		data,
		validation.Parameter(&data.RoomId, validation.Type("string"), validation.Required()),
		validation.Parameter(&data.MessageId, validation.Type("string"), validation.Required()),
		validation.Parameter(&data.AuthorId, validation.Type("string"), validation.Required()),
		validation.Parameter(&data.Message, validation.Type("string"), validation.Required()),
	); err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	// appId := e.GetSessionCache("appId")
	userInfo := e.GetSessionCache("userInfo").(*sso.AnonymousUserInfo)

	if data.AuthorId != userInfo.Uid {
		res.Errors(err)
		res.Code = 10202
		res.CallSocketIo(e)
		return err
	}

	messageId, err := primitive.ObjectIDFromHex(data.MessageId)
	if err != nil {
		res.Errors(err)
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	message := messagesDbx.EditMessage(messageId, data.RoomId, data.AuthorId, data.Message)
	if message == nil {
		res.Errors(err)
		res.Code = 10011
		res.CallSocketIo(e)
		return err
	}

	log.Info("message", message.Message)

	fMessage := methods.FormatMessages([]*models.Messages{message})

	if len(fMessage) == 0 {
		res.Errors(err)
		res.Code = 10011
		res.CallSocketIo(e)
		return err
	}
	responseData := protos.EditMessage_Response{
		Message: fMessage[0],
	}
	res.Code = 200
	res.Data = protos.Encode(&responseData)
	res.CallSocketIo(e)

	msc := methods.SocketConn{
		Conn: e.ConnContext(),
	}

	msc.BroadcastToRoom(data.RoomId,
		routeEventName["receiveEditMessage"],
		&res,
		false)
	return nil
}

// 支持群组多人通话或者个人通话
// groupId string，uids array
func (cc *ChatController) StartCalling(e *nsocketio.EventInstance) error {
	// 1、初始化返回体
	var res response.ResponseProtobufType
	res.Code = 200

	// 2、获取参数
	data := new(protos.StartCalling_Request)
	var err error
	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
		res.Error = err.Error()
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}
	// 3、校验参数
	if err = validation.ValidateStruct(
		data,
		validation.Parameter(&data.RoomId, validation.Required()),
		validation.Parameter(&data.Type, validation.Required(), validation.Enum([]string{"Audio", "Video", "ScreenShare"})),
		validation.Parameter(&data.Participants, validation.Required()),
	); err != nil || len(data.Participants) <= 1 {
		res.Error = err.Error()
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}

	// 3、获取参数
	appId := e.GetSessionCache("appId").(string)
	userInfo := e.GetSessionCache("userInfo").(*sso.AnonymousUserInfo)
	authorId := userInfo.Uid

	// 存储一个临时token到redis,每个用户一个 key由roomId和uid生成
	// 用于校验通话

	rKey := conf.Redisdb.GetKey("SFUCallToken")
	ck := cipher.MD5(data.RoomId + nstrings.ToString(time.Now().Unix()))
	for _, v := range data.Participants {
		err = conf.Redisdb.Set(rKey.GetKey(appId+data.RoomId+v.Uid), ck, rKey.GetExpiration())
		if err != nil {
			res.Error = err.Error()
			res.Code = 10001
			res.CallSocketIo(e)
			return nil
		}
	}

	// fmt.Println(msgRes)
	t := time.Duration(conf.Config.Turn.Auth.Duration) * time.Second

	u, p, err := turn.GenerateLongTermCredentials(conf.Config.Turn.Auth.Secret, t)
	if err != nil {
		res.Error = err.Error()
		res.Code = 10001
		res.CallSocketIo(e)
		return nil
	}

	res.Code = 200
	res.Data = protos.Encode(&protos.StartCalling_Response{
		Participants:  data.Participants,
		RoomId:        data.RoomId,
		Type:          data.Type,
		CurrentUserId: authorId,
		CallToken:     ck,
		TurnServer: &protos.TurnServer{
			Urls: []string{
				conf.Config.Turn.Address,
			},
			Username:   u,
			Credential: p,
		},
	})
	res.CallSocketIo(e)

	msc := methods.SocketConn{
		Conn: e.ConnContext(),
	}
	msc.BroadcastToRoom(data.RoomId,
		routeEventName["startCallingMessage"],
		&res,
		true)

	return nil
}

func (cc *ChatController) Hangup(e *nsocketio.EventInstance) error {
	// 1、初始化返回体
	var res response.ResponseProtobufType
	res.Code = 200

	// 2、校验参数
	data := new(protos.Hangup_Request)
	var err error
	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
		res.Error = err.Error()
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}
	// 3、校验参数
	if err = validation.ValidateStruct(
		data,
		validation.Parameter(&data.RoomId, validation.Required()),
		validation.Parameter(&data.Type, validation.Required(), validation.Enum([]string{"Audio", "Video", "ScreenShare"})),
		validation.Parameter(&data.Participants, validation.Required()),
	); err != nil || len(data.Participants) <= 1 {
		res.Error = err.Error()
		res.Code = 10002
		res.CallSocketIo(e)
		return err
	}
	// 3、获取参数
	userInfo := e.GetSessionCache("userInfo").(*sso.AnonymousUserInfo)
	authorId := userInfo.Uid

	// fmt.Println("fromUid", fromUid, toUids, typeStr)
	// if data.Send {
	// 	// 需要发送信息告知结果
	// 	log.Info("需要发送信息告知结果(预留)")
	// 	if err = validation.ValidateStruct(
	// 		data,
	// 		validation.Parameter(&data.CallTime, validation.Required()),
	// 		validation.Parameter(&data.Status, validation.Required(), validation.Enum([]int64{1, 0, -1, -2, -3})),
	// 	); err != nil {
	// 		res.Error = err.Error()
	// 		res.Code = 10002
	// 		res.CallSocketIo(e)
	// 		return err
	// 	}
	// }

	res.Code = 200

	res.Data = protos.Encode(&protos.Hangup_Response{
		Participants:  data.Participants,
		RoomId:        data.RoomId,
		Type:          data.Type,
		Status:        data.Status,
		CallTime:      data.CallTime,
		CurrentUserId: authorId,
	})
	res.CallSocketIo(e)

	msc := methods.SocketConn{
		Conn: e.ConnContext(),
	}
	msc.BroadcastToRoom(data.RoomId,
		routeEventName["hangupMessage"],
		&res,
		true)

	return nil
}

// func NewChatConnect(e *nsocketio.EventInstance) error {
// 	// getUserInfo := e.GetSessionCache("userInfo")
// 	log.Info("/Chat 开始连接")
// 	// if getUserInfo != nil {
// 	// 	userInfo := getUserInfo.(*sso.UserInfo)
// 	// 	log.Info("/Chat UID", userInfo.Uid)
// 	// 	c.SetCustomId(methods.GetUserRoomId(userInfo.Uid))
// 	// 	return nil
// 	// }

// 	Conn := e.Conn()
// 	c := e.ConnContext()
// 	sc := methods.SocketConn{
// 		Conn: Conn,
// 	}

// 	query := new(typings.SocketEncryptionQuery)

// 	err := qs.Unmarshal(query, Conn.URL().RawQuery)

// 	if err != nil {
// 		// res.Code = 10002
// 		// res.Data = err.Error()
// 		// Conn.Emit(conf.SocketRouterEventNames["Error"], res.GetReponse())
// 		// defer Conn.Close()
// 		// return err
// 	}
// 	sc.Query = query
// 	// log.Info("query", query)

// 	queryData := new(typings.SocketQuery)
// 	deQueryDataErr := sc.Decryption(queryData)
// 	// log.Info("deQueryDataErr", deQueryDataErr != nil, deQueryDataErr)
// 	if deQueryDataErr != nil {
// 		// res.Code = 10009
// 		// res.Data = deQueryDataErr.Error()
// 		// Conn.Emit(conf.SocketRouterEventNames["Error"], res.GetReponse())
// 		// defer Conn.Close()
// 		// return deQueryDataErr
// 	}
// 	// log.Info("queryData", queryData)
// 	getUser, err := conf.SSO.Verify(queryData.Token, queryData.DeviceId, queryData.UserAgent)
// 	if err != nil || getUser == nil || getUser.Payload.Uid == 0 {
// 		// res.Code = 10004
// 		// res.Data = "SSO Error: " + err.Error()
// 		// Conn.Emit(conf.SocketRouterEventNames["Error"], res.GetReponse())
// 		// defer Conn.Close()
// 		return err
// 	} else {
// 		log.Warn(c.Namespace()+" => 连接成功！", c.Conn.ID(), strconv.FormatInt(getUser.Payload.Uid, 10)+", Connection to Successful.")
// 		// c.SetRoomId(nstrings.ToString(getUser.Payload.Uid), getUser.Payload.UserAgent.DeviceId)
// 		// c.SetCustomId(cipher.MD5(nstrings.ToString(getUser.Payload.Uid) + getUser.Payload.UserAgent.DeviceId))
// 		c.SetTag("Uid", nstrings.ToString(getUser.Payload.Uid))
// 		c.SetTag("DeviceId", getUser.Payload.UserAgent.DeviceId)

// 	}
// 	return nil
// }
// func ChatDisconnect(e *nsocketio.EventInstance) error {
// 	c := e.ConnContext()

// 	// 离开了给之前加入的Room发送退出信息
// 	for _, roomId := range c.GetRoomsWithNamespace() {
// 		// log.Info(roomId)
// 		// 1、获取该room下面所有用户
// 		conns := c.GetAllConnContextInRoomWithNamespace(roomId)
// 		// log.Info("获取所有连接", conns)
// 		for _, conn := range conns {
// 			// log.Info(c.Conn.ID(), conn.Conn.ID() == c.Conn.ID())
// 			if conn.ID() == "" || conn.ID() == c.ID() {
// 				continue
// 			}
// 			// 2、获取每个用户的匿名信息
// 			anonymousUserInfo := conn.GetSessionCache("anonymousUserInfo")
// 			auid := int64(0)
// 			// log.Info("anonymousUserInfo", anonymousUserInfo)
// 			if anonymousUserInfo != nil {
// 				// log.Info("anonymousUserInfo", anonymousUserInfo)
// 				auid = anonymousUserInfo.(*sso.UserInfo).Uid
// 			} else {
// 			}
// 			userInfo, isuserInfo := conn.GetSessionCache("userInfo").(*sso.UserInfo)
// 			// log.Info("userInfo", userInfo.Uid)
// 			if !isuserInfo {
// 				continue
// 			}
// 			// log.Info(userInfo.Nickname, userInfo.Uid)
// 			// 3、通过UID获取加密数据并发送出去
// 			var res response.ResponseProtobufType
// 			res.Code = 200
// 			res.Data = protos.Encode(&protos.LeaveRoom_Response{
// 				RoomId:       roomId,
// 				AuthorId:     userInfo.Uid,
// 				AnonymousUID: auid,
// 				TotalUser:    int64(len(conns)),
// 			})

// 			userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, userInfo.UserAgent.DeviceId)
// 			// log.Info("userAesKey", userAesKey)
// 			if userAesKey == nil {
// 				return nil
// 			}
// 			eventName := conf.SocketRouterEventNames["LeaveRoom"]
// 			responseData := res.Encryption(userAesKey.AESKey, res.GetReponse())

// 			// conn := c.GetConnContext(cipher.MD5(nstrings.ToString(userInfo.Uid) + userInfo.UserAgent.DeviceId))
// 			// log.Info("---------------------- OnConnect LeaveRoom", conn, c, "----------------------")
// 			// if conn != nil {

// 			// log.Info("eventName, responseData", eventName, responseData)
// 			isEmit := conn.Emit(eventName, responseData)
// 			// log.Info("isEmit", isEmit)
// 			if isEmit {
// 				// 发送成功或存储到数据库
// 			} else {
// 				// 存储到数据库作为离线数据
// 			}
// 			// }
// 		}

// 	}
// 	getUserInfo := e.GetSessionCache("userInfo")
// 	log.Warn(c.Namespace()+" => 该用户退出前有这些房间", c.GetRoomsWithNamespace())
// 	log.Warn(c.Namespace()+" => 普通聊天离开监听", c.Conn.ID()+"关闭了"+c.Namespace()+"连接：", e.Reason)
// 	log.Warn(c.Namespace() + " => 清理所有socketio缓存")
// 	log.Warn(c.Namespace()+" => 退出原因:", e.Reason)
// 	if getUserInfo != nil {
// 		c.Clear()
// 		c.ClearSessionCache()
// 	}
// 	return nil
// }

// func EmitChatMessageToUser(e *nsocketio.EventInstance, userId int64, chatRecords *models.ChatRecords) {
// 	getConnContext := conf.SocketIO.GetConnContextByTag(conf.SocketRouterNamespace["Chat"], "Uid", nstrings.ToString(userId))

// 	for _, c := range getConnContext {
// 		deviceId := c.GetTag("DeviceId")
// 		userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, deviceId)
// 		if userAesKey == nil {
// 			return
// 		}
// 		// roomId := methods.GetUserRoomId(userId)
// 		eventName := conf.SocketRouterEventNames["ChatMessage"]

// 		var msgRes response.ResponseProtobufType
// 		msgRes.Code = 200
// 		// fmt.Println(msgRes)

// 		protosChatRecordItem := new(protos.ChatRecordItem)
// 		copier.Copy(protosChatRecordItem, chatRecords)
// 		(*protosChatRecordItem).Id = chatRecords.Id.Hex()
// 		if chatRecords.ReplyId != primitive.NilObjectID {
// 			(*protosChatRecordItem).ReplyId = chatRecords.ReplyId.Hex()
// 		}
// 		// protosChatRecordItem.ReadUserIds = append(protosChatRecordItem.ReadUserIds, data.FriendId)
// 		msgRes.Data = protos.Encode(&protos.ChatMessage_Response{
// 			Message: protosChatRecordItem,
// 		})
// 		// fmt.Println("PostChatMessage", "toUid", toUid, "FromUid", userInfo.Uid)

// 		data := msgRes.Encryption(userAesKey.AESKey, msgRes.GetReponse())
// 		isEmit := c.Emit(eventName, data)
// 		log.Info("isEmit", isEmit)
// 		if isEmit {
// 			// 发送成功或存储到数据库
// 			// 发送成功不代表看了。
// 			// chatRecords.ReadUserIds = append(chatRecords.ReadUserIds, data.FriendId)
// 		} else {
// 			// 存储到数据库作为离线数据
// 		}
// 	}
// }

// // 暂时未开发
// func SendChatMessageToRoom(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType

// 	// 2、获取参数
// 	data := new(protos.PostChatMessage_Request)
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	if data.FriendId == 0 && data.GroupId == 0 {
// 		res.Msg = "At least one of friendId and groupId has a value."
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return errors.New("At least one of friendId and groupId has a value.")
// 	}
// 	authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid

// 	// log.Info(data.Message, data.FriendId, data.GroupId, authorId)

// 	// 4、检测互相是否是好友
// 	if data.FriendId != 0 {
// 		getFriend, err := friendsDbx.GetFriendByUserId(authorId, data.FriendId)
// 		// log.Info("getFriend", getFriend)
// 		if len(getFriend) == 0 && err != nil {
// 			res.Code = 10105
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}

// 		chatRecords := new(models.ChatRecords)
// 		chatRecords.AuthorId = authorId
// 		copier.Copy(chatRecords, data)
// 		err = chatRecords.Default()
// 		if err != nil {
// 			res.Code = 10201
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}

// 		var replyId primitive.ObjectID
// 		if data.ReplyId != "" {
// 			replyId, _ = primitive.ObjectIDFromHex(data.ReplyId)
// 			chatRecords.ReplyId = replyId
// 		}
// 		var forwardChatIds []primitive.ObjectID
// 		if data.ForwardChatIds != nil && len(data.ForwardChatIds) != 0 {
// 			for _, v := range data.ForwardChatIds {
// 				oid, _ := primitive.ObjectIDFromHex(v)
// 				forwardChatIds = append(forwardChatIds, oid)
// 			}
// 			chatRecords.ForwardChatIds = forwardChatIds
// 		} else {

// 		}

// 		// if data.Audio != nil {
// 		// 	chatRecords.Audio = models.ChatRecordsAudio{
// 		// 		Time: data.Audio.Time,
// 		// 		Url:  data.Audio.Url,
// 		// 	}
// 		// }
// 		// if data.Video != nil {
// 		// 	chatRecords.Video = models.ChatRecordsVideo{
// 		// 		Time: data.Video.Time,
// 		// 		Url:  data.Video.Url,
// 		// 	}
// 		// }
// 		// if data.Image != nil {
// 		// 	chatRecords.Image = models.ChatRecordsImage{
// 		// 		Url: data.Image.Url,
// 		// 	}
// 		// }
// 		// if data.Call != nil {
// 		// 	chatRecords.Call = models.ChatRecordsCall{
// 		// 		Status: data.Call.Status,
// 		// 		Type:   data.Call.Type,
// 		// 		Time:   data.Call.Time,
// 		// 	}
// 		// }

// 		// log.Info("chatRecords", data, chatRecords)

// 		// 6、存储到数据库
// 		insertedID, err := chatDbx.SaveChatRecord(chatRecords)
// 		if err != nil {
// 			res.Code = 10201
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}
// 		log.Info("insertedID.InsertedID", insertedID.InsertedID)

// 		// 7、记录最新聊天时刻
// 		chatDbx.UpdateLastSeenTime(authorId, data.FriendId)

// 		res.Data = protos.Encode(&protos.PostChatMessage_Response{
// 			Id:          insertedID.InsertedID.(primitive.ObjectID).Hex(),
// 			ReadUserIds: chatRecords.ReadUserIds,
// 		})
// 		res.Code = 200
// 		res.CallSocketIo(e)

// 		// 发送给对方
// 		go EmitChatMessageToUser(e, data.FriendId, chatRecords)
// 		// go func() {
// 		// 	userAesKey, getAesKeyErr := conf.EncryptionClient.GetUserAesKey(strconv.FormatInt(data.FriendId, 10))
// 		// 	if getAesKeyErr != nil {
// 		// 		return
// 		// 	}
// 		// 	roomId := methods.GetUserRoomId(data.FriendId)
// 		// 	eventName := conf.SocketRouterEventNames["ChatMessage"]

// 		// 	var msgRes response.ResponseProtobufType
// 		// 	msgRes.Code = 200
// 		// 	// fmt.Println(msgRes)

// 		// 	protosChatRecordItem := new(protos.ChatRecordItem)
// 		// 	copier.Copy(protosChatRecordItem, chatRecords)
// 		// 	(*protosChatRecordItem).Id = chatRecords.Id.Hex()
// 		// 	if chatRecords.ReplyId != primitive.NilObjectID {
// 		// 		(*protosChatRecordItem).ReplyId = chatRecords.ReplyId.Hex()
// 		// 	}
// 		// 	// protosChatRecordItem.ReadUserIds = append(protosChatRecordItem.ReadUserIds, data.FriendId)
// 		// 	msgRes.Data = protos.Encode(&protos.ChatMessage_Response{
// 		// 		Message: protosChatRecordItem,
// 		// 	})
// 		// 	// fmt.Println("PostChatMessage", "toUid", toUid, "FromUid", userInfo.Uid)

// 		// 	c := socketiomid.ConnContext{
// 		// 		ServerContext: conf.SocketIoServer,
// 		// 	}
// 		// 	log.Info(data.Message, data.FriendId, data.GroupId, authorId)

// 		// 	data := res.Encryption(userAesKey, msgRes.GetReponse())
// 		// 	isEmit := c.Emit(conf.SocketRouterNamespace["Chat"], roomId, eventName, data)
// 		// 	log.Info("isEmit", isEmit)
// 		// 	if isEmit {
// 		// 		// 发送成功或存储到数据库
// 		// 		// 发送成功不代表看了。
// 		// 		// chatRecords.ReadUserIds = append(chatRecords.ReadUserIds, data.FriendId)
// 		// 	} else {
// 		// 		// 存储到数据库作为离线数据
// 		// 	}
// 		// }()
// 	}
// 	if data.GroupId != 0 {
// 		res.Code = 200
// 		res.CallSocketIo(e)
// 	}

// 	return nil
// }

// func PostChatMessage(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType

// 	// 2、获取参数
// 	data := new(protos.PostChatMessage_Request)
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	if data.FriendId == 0 && data.GroupId == 0 {
// 		res.Msg = "At least one of friendId and groupId has a value."
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return errors.New("At least one of friendId and groupId has a value.")
// 	}
// 	authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid

// 	// log.Info(data.Message, data.FriendId, data.GroupId, authorId)

// 	// 4、检测互相是否是好友
// 	if data.FriendId != 0 {
// 		getFriend, err := friendsDbx.GetFriendByUserId(authorId, data.FriendId)
// 		// log.Info("getFriend", getFriend)
// 		if len(getFriend) == 0 && err != nil {
// 			res.Code = 10105
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}

// 		chatRecords := new(models.ChatRecords)
// 		chatRecords.AuthorId = authorId
// 		copier.Copy(chatRecords, data)
// 		err = chatRecords.Default()
// 		if err != nil {
// 			res.Code = 10201
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}

// 		var replyId primitive.ObjectID
// 		if data.ReplyId != "" {
// 			replyId, _ = primitive.ObjectIDFromHex(data.ReplyId)
// 			chatRecords.ReplyId = replyId
// 		}
// 		var forwardChatIds []primitive.ObjectID
// 		if data.ForwardChatIds != nil && len(data.ForwardChatIds) != 0 {
// 			for _, v := range data.ForwardChatIds {
// 				oid, _ := primitive.ObjectIDFromHex(v)
// 				forwardChatIds = append(forwardChatIds, oid)
// 			}
// 			chatRecords.ForwardChatIds = forwardChatIds
// 		} else {

// 		}

// 		// if data.Audio != nil {
// 		// 	chatRecords.Audio = models.ChatRecordsAudio{
// 		// 		Time: data.Audio.Time,
// 		// 		Url:  data.Audio.Url,
// 		// 	}
// 		// }
// 		// if data.Video != nil {
// 		// 	chatRecords.Video = models.ChatRecordsVideo{
// 		// 		Time: data.Video.Time,
// 		// 		Url:  data.Video.Url,
// 		// 	}
// 		// }
// 		// if data.Image != nil {
// 		// 	chatRecords.Image = models.ChatRecordsImage{
// 		// 		Url: data.Image.Url,
// 		// 	}
// 		// }
// 		// if data.Call != nil {
// 		// 	chatRecords.Call = models.ChatRecordsCall{
// 		// 		Status: data.Call.Status,
// 		// 		Type:   data.Call.Type,
// 		// 		Time:   data.Call.Time,
// 		// 	}
// 		// }

// 		// log.Info("chatRecords", data, chatRecords)

// 		// 6、存储到数据库
// 		insertedID, err := chatDbx.SaveChatRecord(chatRecords)
// 		if err != nil {
// 			res.Code = 10201
// 			res.Error = err.Error()
// 			res.CallSocketIo(e)
// 			return err
// 		}
// 		log.Info("insertedID.InsertedID", insertedID.InsertedID)

// 		// 7、记录最新聊天时刻
// 		chatDbx.UpdateLastSeenTime(authorId, data.FriendId)

// 		res.Data = protos.Encode(&protos.PostChatMessage_Response{
// 			Id:          insertedID.InsertedID.(primitive.ObjectID).Hex(),
// 			ReadUserIds: chatRecords.ReadUserIds,
// 		})
// 		res.Code = 200
// 		res.CallSocketIo(e)

// 		// 发送给对方
// 		go EmitChatMessageToUser(e, data.FriendId, chatRecords)
// 		// go func() {
// 		// 	userAesKey, getAesKeyErr := conf.EncryptionClient.GetUserAesKey(strconv.FormatInt(data.FriendId, 10))
// 		// 	if getAesKeyErr != nil {
// 		// 		return
// 		// 	}
// 		// 	roomId := methods.GetUserRoomId(data.FriendId)
// 		// 	eventName := conf.SocketRouterEventNames["ChatMessage"]

// 		// 	var msgRes response.ResponseProtobufType
// 		// 	msgRes.Code = 200
// 		// 	// fmt.Println(msgRes)

// 		// 	protosChatRecordItem := new(protos.ChatRecordItem)
// 		// 	copier.Copy(protosChatRecordItem, chatRecords)
// 		// 	(*protosChatRecordItem).Id = chatRecords.Id.Hex()
// 		// 	if chatRecords.ReplyId != primitive.NilObjectID {
// 		// 		(*protosChatRecordItem).ReplyId = chatRecords.ReplyId.Hex()
// 		// 	}
// 		// 	// protosChatRecordItem.ReadUserIds = append(protosChatRecordItem.ReadUserIds, data.FriendId)
// 		// 	msgRes.Data = protos.Encode(&protos.ChatMessage_Response{
// 		// 		Message: protosChatRecordItem,
// 		// 	})
// 		// 	// fmt.Println("PostChatMessage", "toUid", toUid, "FromUid", userInfo.Uid)

// 		// 	c := socketiomid.ConnContext{
// 		// 		ServerContext: conf.SocketIoServer,
// 		// 	}
// 		// 	log.Info(data.Message, data.FriendId, data.GroupId, authorId)

// 		// 	data := res.Encryption(userAesKey, msgRes.GetReponse())
// 		// 	isEmit := c.Emit(conf.SocketRouterNamespace["Chat"], roomId, eventName, data)
// 		// 	log.Info("isEmit", isEmit)
// 		// 	if isEmit {
// 		// 		// 发送成功或存储到数据库
// 		// 		// 发送成功不代表看了。
// 		// 		// chatRecords.ReadUserIds = append(chatRecords.ReadUserIds, data.FriendId)
// 		// 	} else {
// 		// 		// 存储到数据库作为离线数据
// 		// 	}
// 		// }()
// 	}
// 	if data.GroupId != 0 {
// 		res.Code = 200
// 		res.CallSocketIo(e)
// 	}

// 	return nil
// }

// // 支持群组多人通话或者个人通话
// // groupId string，uids array
// func StartCalling(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、获取参数
// 	data := new(protos.StartCalling_Request)
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Error = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	if err = validation.ValidateStruct(
// 		data,
// 		validation.Parameter(&data.Type, validation.Required(), validation.Enum([]string{"Audio", "Video", "ScreenShare"})),
// 		validation.Parameter(&data.AuthorId, validation.Required(), validation.GreaterEqual(1)),
// 		validation.Parameter(&data.Participants, validation.Required()),
// 	); err != nil || len(data.Participants) == 0 {
// 		res.Error = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// 3、获取参数
// 	userInfo := e.GetSessionCache("userInfo").(*sso.UserInfo)
// 	authorId := userInfo.Uid

// 	// fmt.Println("fromUid", fromUid, toUids, typeStr)
// 	userIds := []int64{}
// 	participants := []models.ChatRecordsCallParticipants{}
// 	for _, uid := range data.Participants {
// 		userIds = append(userIds, uid.Uid)
// 		participants = append(participants, models.ChatRecordsCallParticipants{
// 			Uid: uid.Uid,
// 		})
// 	}
// 	userIds = methods.DedupeArrayInt64(userIds)

// 	callRoomId := data.RoomId
// 	if data.RoomId == "" {
// 		callRoomId = methods.GetCallUserRoomId(
// 			append(userIds, data.GroupId, time.Now().Unix()),
// 		)
// 	}
// 	log.Info("callRoomId", callRoomId, data.RoomId)
// 	// callRoomId := "F6CB3F389A43D768F4B152C3B2D23DD4"

// 	res.Code = 200
// 	// fmt.Println(msgRes)
// 	res.Data = protos.Encode(&protos.StartCalling_Response{
// 		AuthorId:      data.AuthorId,
// 		Participants:  data.Participants,
// 		GroupId:       data.GroupId,
// 		Type:          data.Type,
// 		RoomId:        callRoomId,
// 		CurrentUserId: authorId,
// 	})
// 	res.CallSocketIo(e)
// 	go func() {
// 		// 发送给群
// 		if data.GroupId != 0 {

// 		} else {

// 			for _, user := range data.Participants {
// 				if data.AuthorId != user.Uid {

// 					getConnContext := e.ServerContext().GetConnContextByTag(conf.SocketRouterNamespace["Chat"], "Uid", nstrings.ToString(user.Uid))

// 					for _, c := range getConnContext {
// 						deviceId := c.GetTag("DeviceId")
// 						userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, deviceId)
// 						if userAesKey == nil {
// 							return
// 						}

// 						eventName := conf.SocketRouterEventNames["StartCallingMessage"]
// 						responseData := res.Encryption(userAesKey.AESKey, res.GetReponse())

// 						isEmit := c.Emit(eventName, responseData)
// 						if isEmit {
// 							// 发送成功或存储到数据库
// 						} else {
// 							// 存储到数据库作为离线数据
// 						}

// 					}

// 					// 发送给个人信息
// 					// if data.GroupId == 0 {
// 					// 	// 检测是否发过，以防万一。
// 					// 	getCR, err := chatDbx.GetChatRecordsByCallInfo(data.AuthorId, user.Uid, callRoomId, -3)

// 					// 	log.Info("err:", getCR, err, err != nil)
// 					// 	if len(getCR) > 0 || err != nil {
// 					// 		return
// 					// 	}
// 					// 	chatRecords := new(models.ChatRecords)
// 					// 	chatRecords.AuthorId = data.AuthorId
// 					// 	chatRecords.FriendId = user.Uid

// 					// 	chatRecords.Call = models.ChatRecordsCall{
// 					// 		Status:       -3,
// 					// 		RoomId:       callRoomId,
// 					// 		Participants: participants,
// 					// 		GroupId:      data.GroupId,
// 					// 		AuthorId:     data.AuthorId,
// 					// 		Type:         data.Type,
// 					// 		Time:         0,
// 					// 	}

// 					// 	err = chatRecords.Default()
// 					// 	if err != nil {
// 					// 		log.Info(err)
// 					// 		return
// 					// 	}
// 					// 	// 6、存储到数据库
// 					// 	_, err = chatDbx.SaveChatRecord(chatRecords)
// 					// 	if err != nil {
// 					// 		res.Code = 10201
// 					// 		res.Error = err.Error()
// 					// 		res.CallSocketIo(e)
// 					// 		return
// 					// 	}
// 					// 	// 7、记录最新聊天时刻
// 					// 	chatDbx.UpdateLastSeenTime(authorId, user.Uid)

// 					// 	go EmitChatMessageToUser(c, authorId, chatRecords)
// 					// 	go EmitChatMessageToUser(c, user.Uid, chatRecords)
// 					// }
// 				}
// 			}
// 		}
// 	}()

// 	return nil
// }

// func Hangup(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、校验参数
// 	data := new(protos.Hangup_Request)
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Error = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	if err = validation.ValidateStruct(
// 		data,
// 		validation.Parameter(&data.Type, validation.Required(), validation.Enum([]string{"Audio", "Video", "ScreenShare"})),
// 		validation.Parameter(&data.Participants, validation.Required()),
// 		validation.Parameter(&data.AuthorId, validation.Required(), validation.GreaterEqual(1)),
// 		validation.Parameter(&data.RoomId, validation.Required()),
// 	); err != nil || len(data.Participants) == 0 {
// 		res.Error = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、获取参数
// 	userInfo := e.GetSessionCache("userInfo").(*sso.UserInfo)
// 	authorId := userInfo.Uid

// 	// fmt.Println("fromUid", fromUid, toUids, typeStr)
// 	userIds := []int64{}
// 	participants := []models.ChatRecordsCallParticipants{}
// 	for _, uid := range data.Participants {
// 		userIds = append(userIds, uid.Uid)
// 		participants = append(participants, models.ChatRecordsCallParticipants{
// 			Uid: uid.Uid,
// 		})
// 	}
// 	userIds = methods.DedupeArrayInt64(userIds)

// 	res.Data = protos.Encode(&protos.Hangup_Response{
// 		AuthorId:      data.AuthorId,
// 		Participants:  data.Participants,
// 		GroupId:       data.GroupId,
// 		Type:          data.Type,
// 		RoomId:        data.RoomId,
// 		CurrentUserId: authorId,
// 	})
// 	res.CallSocketIo(e)

// 	go func() {
// 		// 发送给群
// 		if data.GroupId != 0 {

// 		} else {
// 			for _, user := range data.Participants {
// 				if data.AuthorId != user.Uid {

// 					getConnContext := e.ServerContext().GetConnContextByTag(conf.SocketRouterNamespace["Chat"], "Uid", nstrings.ToString(user.Uid))

// 					for _, c := range getConnContext {
// 						deviceId := c.GetTag("DeviceId")
// 						userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, deviceId)
// 						if userAesKey == nil {
// 							return
// 						}

// 						eventName := conf.SocketRouterEventNames["HangupMessage"]
// 						responseData := res.Encryption(userAesKey.AESKey, res.GetReponse())

// 						isEmit := c.Emit(eventName, responseData)
// 						if isEmit {
// 							// 发送成功或存储到数据库
// 						} else {
// 							// 存储到数据库作为离线数据
// 						}

// 					}

// 					log.Info("data.Send", data.Send)
// 					// 发送给个人信息
// 					// if data.GroupId == 0 && data.Send && data.Status == 1 && data.CallTime > 0 {
// 					// 	// 检测是否发过，以防万一。
// 					// 	getCR, err := chatDbx.GetChatRecordsByCallInfo(data.AuthorId, user.Uid, data.RoomId, 1)

// 					// 	// log.Info("err:", getCR, err, err != nil)
// 					// 	if len(getCR) > 0 || err != nil {
// 					// 		return
// 					// 	}
// 					// 	chatRecords := new(models.ChatRecords)
// 					// 	chatRecords.AuthorId = data.AuthorId
// 					// 	chatRecords.FriendId = user.Uid

// 					// 	chatRecords.Call = models.ChatRecordsCall{
// 					// 		Status:       1,
// 					// 		RoomId:       data.RoomId,
// 					// 		Participants: participants,
// 					// 		GroupId:      data.GroupId,
// 					// 		AuthorId:     data.AuthorId,
// 					// 		Type:         data.Type,
// 					// 		Time:         data.CallTime,
// 					// 	}

// 					// 	err = chatRecords.Default()
// 					// 	if err != nil {
// 					// 		log.Info(err)
// 					// 		return
// 					// 	}

// 					// 	// 6、存储到数据库
// 					// 	_, err = chatDbx.SaveChatRecord(chatRecords)
// 					// 	if err != nil {
// 					// 		res.Code = 10201
// 					// 		res.Error = err.Error()
// 					// 		res.CallSocketIo(e)
// 					// 		return
// 					// 	}
// 					// 	// 7、记录最新聊天时刻
// 					// 	chatDbx.UpdateLastSeenTime(authorId, user.Uid)

// 					// 	go EmitChatMessageToUser(c, authorId, chatRecords)
// 					// 	go EmitChatMessageToUser(c, user.Uid, chatRecords)
// 					// }
// 				}

// 			}
// 		}
// 	}()

// 	return nil
// }

// func DeleteDialog(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、获取参数
// 	data := new(protos.DeleteDialog_Request)
// 	authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	if data.FriendId == 0 && data.GroupId == 0 {
// 		res.Msg = "At least one of friendId and groupId has a value."
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return errors.New("At least one of friendId and groupId has a value.")
// 	}

// 	// 4、操作数据库
// 	if data.FriendId != 0 {
// 		_, err = chatDbx.DeleteUserDialog(data.FriendId, authorId)
// 	}
// 	if data.GroupId != 0 {
// 		// 群
// 	}
// 	if err != nil {
// 		res.Code = 10011
// 		res.Msg = err.Error()
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// 5、解析数据
// 	res.Data = protos.Encode(&protos.DeleteDialog_Response{})

// 	res.CallSocketIo(e)
// 	return nil
// }

// func GetUnreadChatRecords(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、校验参数
// 	err := validationv2.Validation(
// 		validationv2.Field("id", e.GetData("id"), validationv2.Required()),
// 		validationv2.Field("pageNum", e.GetData("pageNum"), validationv2.Required()),
// 		validationv2.Field("pageSize", e.GetData("pageSize"), validationv2.Required()),
// 		validationv2.Field("type", e.GetString("type"), validationv2.Required()),
// 		validationv2.Field("lastChatTime", e.GetData("lastChatTime"), validationv2.Required()),
// 	)
// 	if err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// // 3、获取参数
// 	userInfo := e.GetSessionCache("userInfo").(*sso.UserInfo)
// 	id := int64(e.GetData("id").(float64))
// 	pageNum := int64(e.GetData("pageNum").(float64))
// 	pageSize := int64(e.GetData("pageSize").(float64))
// 	lastChatTime := int64(e.GetData("lastChatTime").(float64))
// 	typeStr := e.GetString("type")

// 	log.Info("GetUnreadChatRecords", pageNum, pageSize, typeStr, id, userInfo.Uid, lastChatTime)

// 	// 4、获取数据
// 	result, err := chatDbx.GetUnreadChatRecords(pageNum, pageSize, typeStr, id, userInfo.Uid, lastChatTime)
// 	if err != nil {
// 		res.Code = 10006
// 		res.Msg = err.Error()
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// log.Info("result.List", result, result.List, result.UnreadTotal, len(result.List) != 0)

// 	// 5、解析数据
// 	getData := protos.GetUnreadChatRecords_Response{
// 		List:        []*protos.ChatRecordItem{},
// 		UnreadCount: 0,
// 	}
// 	// var oids []primitive.ObjectID = []primitive.ObjectID{}

// 	if len(result.List) != 0 {
// 		for _, v := range result.List {
// 			// oids = append(oids, v.Id)
// 			pcr := new(protos.ChatRecordItem)
// 			copier.Copy(pcr, v)
// 			(*pcr).Id = v.Id.Hex()
// 			if v.ReplyId != primitive.NilObjectID {
// 				(*pcr).ReplyId = v.ReplyId.Hex()
// 			}
// 			getData.List = append(getData.List, pcr)
// 		}
// 	}
// 	// 4、操作数据库
// 	// updateResult, err := chatDbx.ReadChatRecords(oids, userInfo.Uid)
// 	// fmt.Println("updateResult", updateResult, updateResult.ModifiedCount, err)

// 	// if len(result.UnreadTotal) != 0 {
// 	// 	getData.UnreadCount = result.UnreadTotal[0].Count - updateResult.ModifiedCount
// 	// }
// 	// 6、将该内容均设置为已读（或另开接口）

// 	// log.Info("unread", result)
// 	res.Data = protos.Encode(&getData)

// 	res.CallSocketIo(e)
// 	return nil
// }

// func GetChatRecords(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、获取参数
// 	data := new(protos.GetChatRecords_Request)
// 	authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid
// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// log.Info("GetChatRecords", data, len(data.TimeRange) != 0, data.TimeRange[0], data.TimeRange[1])
// 	// 3、校验参数

// 	if err = validation.ValidateStruct(
// 		data,
// 		validation.Parameter(&data.PageNum, validation.Type("int64"), validation.Required(), validation.GreaterEqual(1)),
// 		validation.Parameter(&data.PageSize, validation.Type("int64"), validation.Required(), validation.NumRange(1, 51)),
// 		// validation.Parameter(&data.Type, validation.Type("string"),
// 		// 	validation.Required(),
// 		// 	validation.Enum([]string{"Unread", "All"})),
// 	); err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	if data.FriendId == 0 && data.GroupId == 0 {
// 		res.Msg = "At least one of friendId and groupId has a value."
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return errors.New("At least one of friendId and groupId has a value.")
// 	}

// 	var startTime int64 = 0
// 	var endTime int64 = 0
// 	if len(data.TimeRange) != 0 {
// 		startTime = data.TimeRange[0]
// 		endTime = data.TimeRange[1]
// 	}

// 	// 4、获取数据
// 	result, err := chatDbx.GetChatRecords(authorId, data.FriendId, data.GroupId, data.PageNum, data.PageSize, startTime, endTime)
// 	if err != nil {
// 		res.Code = 10006
// 		res.Msg = err.Error()
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// log.Info("result.List", result, result.List, result.Total, len(result.List) != 0)

// 	// 5、解析数据
// 	getData := protos.GetChatRecords_Response{
// 		List:  []*protos.ChatRecordItem{},
// 		Total: 0,
// 	}
// 	// var oids []primitive.ObjectID = []primitive.ObjectID{}

// 	if len(result.List) != 0 {
// 		for _, v := range result.List {
// 			// oids = append(oids, v.Id)
// 			pcr := new(protos.ChatRecordItem)
// 			copier.Copy(pcr, v)
// 			(*pcr).Id = v.Id.Hex()
// 			if v.ReplyId != primitive.NilObjectID {
// 				(*pcr).ReplyId = v.ReplyId.Hex()
// 			}
// 			getData.List = append(getData.List, pcr)
// 		}
// 	}

// 	res.Data = protos.Encode(&getData)

// 	res.CallSocketIo(e)
// 	return nil
// }

// func GetAllUnreadCount(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	res.Code = 200

// 	// 2、获取参数
// 	// log.Info("user", e.GetSessionCache("userInfo"))
// 	userInfo := e.GetSessionCache("userInfo")
// 	if userInfo == nil {
// 		res.Code = 10004
// 		res.CallSocketIo(e)
// 		return nil
// 	}
// 	authorId := userInfo.(*sso.UserInfo).Uid
// 	// uid := int64(e.GetData("uid").(float64))
// 	// 3、获取数据
// 	getAllUserUnreadMsg, err := chatDbx.GetAllUnreadCount(authorId)
// 	if err != nil {
// 		// Log.Error(err)
// 		res.Code = 10006
// 		res.Error = err.Error()
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// log.Info("getAllUserUnreadMsg", getAllUserUnreadMsg)
// 	// 4、解析数据
// 	getAllUnreadCount := protos.GetAllUnreadCount_Response{
// 		List:  []*protos.UnreadCountItem{},
// 		Total: int64(len(*getAllUserUnreadMsg)),
// 	}

// 	friendCountTempMap := map[int64]int64{}
// 	friendLastMessageTempMap := map[int64]*models.ChatRecords{}

// 	for _, v := range *getAllUserUnreadMsg {
// 		// log.Info("v", v)
// 		if v.GroupId != 0 {
// 		} else {
// 			// if friendCountTempMap[v.AuthorId] == 0 {
// 			friendLastMessageTempMap[v.AuthorId] = v
// 			// }
// 			friendCountTempMap[v.AuthorId]++
// 		}
// 	}

// 	for k, v := range friendCountTempMap {
// 		pcr := new(protos.ChatRecordItem)
// 		copier.Copy(pcr, friendLastMessageTempMap[k])
// 		(*pcr).Id = friendLastMessageTempMap[k].Id.Hex()
// 		getAllUnreadCount.List = append(getAllUnreadCount.List, &protos.UnreadCountItem{
// 			Id:          k,
// 			Type:        "friend",
// 			Count:       v,
// 			LastMessage: pcr,
// 		})
// 	}

// 	res.Data = protos.Encode(&getAllUnreadCount)
// 	// log.Info("getAllUnreadCount1", getAllUnreadCount)
// 	res.CallSocketIo(e)
// 	return nil
// }

// // 每次已阅读，要返id和已有的访问数组
// func ReadChatRecords(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType
// 	// log.Info("ReadChatRecords")

// 	// 2、获取参数
// 	data := new(protos.ReadChatRecords_Request)
// 	authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid

// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil && len(data.Ids) == 0 {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	err = validationv2.Validation(
// 		validationv2.Field("ids", data.Ids, validationv2.Required()),
// 	)
// 	if err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、获取参数
// 	// uid := int64(e.GetData("uid").(float64))
// 	// fmt.Println("ids", data.Ids, authorId)
// 	var oidsOfGetList []primitive.ObjectID = []primitive.ObjectID{}
// 	for _, v := range data.Ids {
// 		oid, _ := primitive.ObjectIDFromHex(v)
// 		oidsOfGetList = append(oidsOfGetList, oid)
// 	}
// 	// 4、获取数据
// 	getList, err := chatDbx.GetChatRecordsListById(oidsOfGetList)
// 	if len(getList) == 0 && err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10011
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// log.Info("getList", getList)

// 	oids := []primitive.ObjectID{}
// 	chatRecordsList := []*models.ChatRecords{}
// 	for _, v := range getList {
// 		// log.Info(v.AuthorId != authorId, !narrays.IncludesInt64(v.ReadUserIds, authorId))
// 		if v.AuthorId != authorId && !narrays.IncludesInt64(v.ReadUserIds, authorId) {
// 			oids = append(oids, v.Id)
// 			chatRecordsList = append(chatRecordsList, v)
// 		}
// 	}
// 	// log.Info("oids", oids)
// 	if len(oids) == 0 {
// 		res.Code = 10012
// 		res.CallSocketIo(e)
// 		return nil
// 	}

// 	// 5、操作数据库
// 	updateResult, err := chatDbx.UpdateManyReadChatRecords(oids, authorId)
// 	if updateResult.MatchedCount != updateResult.ModifiedCount || err != nil {
// 		if updateResult.MatchedCount != updateResult.ModifiedCount {
// 			// 未来加个事务操作
// 		}
// 		res.Msg = err.Error()
// 		res.Code = 10001
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// log.Info("updateResult", updateResult, updateResult, err)

// 	// 6、解析数据
// 	sendList := map[int64]([]*protos.ReadChatRecords_Response_Messages){}
// 	responseList := []*protos.ReadChatRecords_Response_Messages{}
// 	for _, v := range chatRecordsList {
// 		result := protos.ReadChatRecords_Response_Messages{
// 			Id:          v.Id.Hex(),
// 			AuthorId:    v.AuthorId,
// 			FriendId:    v.FriendId,
// 			GroupId:     v.GroupId,
// 			ReadUserIds: append(v.ReadUserIds, authorId),
// 		}
// 		responseList = append(responseList, &result)
// 		if v.FriendId != 0 {
// 			if sendList[v.AuthorId] == nil {
// 				sendList[v.AuthorId] = []*protos.ReadChatRecords_Response_Messages{}
// 			}
// 			sendList[v.AuthorId] = append(sendList[v.AuthorId], &result)
// 		}
// 		if v.GroupId != 0 {
// 			// 待命
// 		}
// 	}

// 	// 7、返回数据
// 	resData := new(protos.ReadChatRecords_Response)
// 	resData.List = responseList
// 	resData.Total = int64(len(responseList))
// 	// log.Info("getAllUnreadCount1", getAllUnreadCount)
// 	res.Data = protos.Encode(resData)

// 	res.Code = 200
// 	res.CallSocketIo(e)

// 	// 8、发送数据

// 	go func() {
// 		for k, v := range sendList {

// 			getConnContext := e.ServerContext().GetConnContextByTag(conf.SocketRouterNamespace["Chat"], "Uid", nstrings.ToString(k))

// 			for _, c := range getConnContext {
// 				deviceId := c.GetTag("DeviceId")
// 				userAesKey := conf.EncryptionClient.GetUserAesKeyByDeviceId(conf.Redisdb, deviceId)
// 				if userAesKey == nil {
// 					return
// 				}

// 				var msgRes response.ResponseProtobufType
// 				msgRes.Code = 200
// 				msgRes.Data = protos.Encode(&protos.ReadChatRecords_Response{
// 					List:  v,
// 					Total: int64(len(v)),
// 				})

// 				// roomId := methods.GetUserRoomId(k)
// 				// userAesKey := e.GetString("userAesKey")

// 				isEmit := c.Emit("ReadMessage", res.Encryption(userAesKey.AESKey, msgRes.GetReponse()))
// 				if isEmit {
// 				} else {
// 				}

// 			}
// 		}
// 	}()
// 	return nil
// }

// func GetChatRecordsReadStatus(e *nsocketio.EventInstance) error {
// 	// 1、初始化返回体
// 	var res response.ResponseProtobufType

// 	// 2、获取参数
// 	data := new(protos.GetChatRecordsReadStatus_Request)
// 	// authorId := e.GetSessionCache("userInfo").(*sso.UserInfo).Uid

// 	var err error
// 	if err = protos.DecodeBase64(e.GetString("data"), data); err != nil && len(data.Ids) == 0 {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、校验参数
// 	err = validationv2.Validation(
// 		validationv2.Field("ids", data.Ids, validationv2.Required()),
// 	)
// 	if err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10002
// 		res.CallSocketIo(e)
// 		return err
// 	}
// 	// 3、获取参数
// 	// uid := int64(e.GetData("uid").(float64))
// 	// fmt.Println("ids", data.Ids, authorId)
// 	var oidsOfGetList []primitive.ObjectID = []primitive.ObjectID{}
// 	for _, v := range data.Ids {
// 		oid, _ := primitive.ObjectIDFromHex(v)
// 		oidsOfGetList = append(oidsOfGetList, oid)
// 	}
// 	// 4、获取数据
// 	getList, err := chatDbx.GetChatRecordsListById(oidsOfGetList)
// 	if len(getList) == 0 && err != nil {
// 		res.Msg = err.Error()
// 		res.Code = 10006
// 		res.CallSocketIo(e)
// 		return err
// 	}

// 	// 5、解析数据
// 	responseList := []*protos.GetChatRecordsReadStatus_Response_Messages{}
// 	for _, v := range getList {
// 		result := protos.GetChatRecordsReadStatus_Response_Messages{
// 			Id:          v.Id.Hex(),
// 			AuthorId:    v.AuthorId,
// 			FriendId:    v.FriendId,
// 			GroupId:     v.GroupId,
// 			ReadUserIds: v.ReadUserIds,
// 		}
// 		responseList = append(responseList, &result)
// 	}

// 	// 7、返回数据
// 	resData := new(protos.GetChatRecordsReadStatus_Response)
// 	resData.List = responseList
// 	resData.Total = int64(len(responseList))
// 	res.Data = protos.Encode(resData)

// 	res.Code = 200
// 	res.CallSocketIo(e)
// 	return nil
// }

// // func JoinRoom(e *nsocketio.EventInstance) error {
// // 	// fmt.Println("开始加入房间")
// // 	// 1、初始化返回体
// // 	var res response.ResponseProtobufType
// // 	res.Code = 200

// // 	// 2、校验参数
// // 	err := validationv2.Validation(
// // 		validationv2.Field("userInfo", e.GetSessionCache("userInfo"), validationv2.Required()),
// // 		validationv2.Field("friendUid", c.GetFloat64("friendUid"), validationv2.Required()),
// // 	)
// // 	// fmt.Println("err", err)
// // 	if err != nil {
// // 		res.Msg = err.Error()
// // 		res.Code = 10002
// // 		res.CallSocketIo(e)
// // 		return err
// // 	}

// // 	// 3、获取参数
// // 	userInfo := e.GetSessionCache("userInfo").(*sso.UserInfo)
// // 	// fmt.Println("userInfo", userInfo)
// // 	friendUid := int64(c.GetFloat64("friendUid"))
// // 	// fmt.Println("friendUid", friendUid)

// // 	// 4、判断互相是否是好友

// // 	// 5、生成rommId
// // 	// friendUserName := data["username"].(string)
// // 	// fmt.Println(msg, friendUserName, friendUid)
// // 	userIds := []int64{friendUid, userInfo.Uid}

// // 	roomId := methods.GetRoomId(c.EventName(), userIds)
// // 	c.Conn.Join(roomId)

// // 	res.Data = protos.Encode(&protos.JoinRoom_Response{
// // 		RoomId: roomId,
// // 	})
// // 	// fmt.Println(res)
// // 	res.CallSocketIo(e)
// // 	return nil
// // }
