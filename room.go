package main

import (
	"context"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/vtb-link/bianka/live"
	"github.com/vtb-link/bianka/proto"
	"log"
	"time"
)

const (
	AccessKey       = "" //access_key
	AccessKeySecret = "" //access_key_secret
	AppId           = 0  // 应用id
)

// InitLive 初始化直播
func InitLive() *LiveRoom {
	room := &LiveRoom{
		DMChan:      make(chan []byte, 128),
		heartbeatTK: time.NewTicker(time.Second * 20),
	}
	sdkConfig := live.NewConfig(AccessKey, AccessKeySecret, int64(AppId))
	// 创建sdk实例
	room.sdk = live.NewClient(sdkConfig)
	// app start
	startResp, err := room.sdk.AppStart(IdCode)
	if err != nil {
		log.Fatal(err)
	}
	room.GameID = startResp.GameInfo.GameID

	go room.heartbeatDaemon()

	// 开启与直播间的连接，并设置消息处理器
	room.client, err = room.sdk.StartWebsocket(startResp, map[uint32]live.DispatcherHandle{proto.OperationMessage: room.handleDM}, nil)
	if err != nil {
		log.Fatal(err)
	}

	//go func() {
	//	str := []string{
	//		`{"cmd":"LIVE_OPEN_PLATFORM_LIKE","data":{"uname":"哔哩哔哩直播","uid":9617619,"uface":"https://i0.hdslb.com/bfs/face/8f6a614a48a3813d90da7a11894ae56a59396fcd.jpg","timestamp":1685946262,"like_text":"为主播点赞了","like_count":114,"fans_medal_wearing_status":false,"fans_medal_name":"","fans_medal_level":0,"msg_id":"57a7c676-ff00-4967-bb09-03e800ab0f4d","room_id":1}}`,
	//		`{"cmd":"LIVE_OPEN_PLATFORM_DM","data":{"emoji_img_url":"","fans_medal_level":0,"fans_medal_name":"","fans_medal_wearing_status":false,"guard_level":0,"msg":"123","timestamp":1702549079,"uid":759939,"uname":"Cliven_","uface":"https://i0.hdslb.com/bfs/face/faff710e724d6ed48d9ab7de1e0c45b9d232e7e8.jpg","dm_type":0,"msg_id":"dadbb4d3-bbd1-41de-819c-b6a46da80619","room_id":22219}}`,
	//		`{"cmd":"LIVE_OPEN_PLATFORM_GUARD","data":{"user_info":{"uid":110000331,"uname":"测试用户","uface":"http://i0.hdslb.com/bfs/face/4add3acfc930fcd07d06ea5e10a3a377314141c2.jpg"},"guard_level":3,"guard_num":1,"guard_unit":"月","fans_medal_level":24,"fans_medal_name":"aw4ifC","fans_medal_wearing_status":false,"timestamp":1653555128,"room_id":460695,"msg_id":""}}`,
	//		`{"cmd":"LIVE_OPEN_PLATFORM_SEND_GIFT","data":{"room_id":1,"uname":"哔哩哔哩直播","uid":9617619,"uface":"https://i0.hdslb.com/bfs/face/8f6a614a48a3813d90da7a11894ae56a59396fcd.jpg","gift_id":0,"gift_name":"爆出道具名","gift_num":11,"price":0,"paid":false,"fans_medal_level":0,"fans_medal_name":"粉丝勋章名","fans_medal_wearing_status":true,"guard_level":0,"timestamp":0,"msg_id":"","anchor_info":{"uid":0,"uname":"","uface":"http://i0.hdslb.com/bfs/face/4add3acfc930fcd07d06ea5e10a3a377314141c2.jpg"},"gift_icon":"http://i1.hdslb.com/dksldksldksld.jpg","combo_gift":true,"combo_info":{"combo_base_num":5,"combo_count":100,"combo_id":"xxxxxx","combo_timeout":3}}}`,
	//		`{"cmd":"LIVE_OPEN_PLATFORM_SUPER_CHAT","data":{"room_id":1,"uname":"哔哩哔哩直播","uid":9617619,"uface":"https://i0.hdslb.com/bfs/face/8f6a614a48a3813d90da7a11894ae56a59396fcd.jpg","message_id":0,"message":"你好","msg_id":"","rmb":10,"timestamp":0,"start_time":0,"end_time":0,"guard_level":2,"fans_medal_level":26,"fans_medal_name":"aw4ifC","fans_medal_wearing_status":true}}`,
	//	}
	//	rand.Seed(uint64(time.Now().Unix()))
	//	for {
	//		log.Println("send!")
	//		time.Sleep(time.Second * 1)
	//		i := rand.Intn(len(str))
	//		room.DMChan <- []byte(str[i])
	//	}
	//}()
	return room
}

// LiveRoom 直播房间
type LiveRoom struct {
	sdk            *live.Client
	client         *live.WsClient
	GameID         string             // 游戏ID
	currentUser    *websocket.Conn    // 当前用户
	lastUserCancel context.CancelFunc // 上一个用户的取消函数
	DMChan         chan []byte        // 弹幕通道
	heartbeatTK    *time.Ticker       // 心跳触发器

}

// Register 注册路由
func (l *LiveRoom) Register(app *fiber.App) {
	// 弹幕消息通道
	app.Get("/dm", websocket.New(l.handleUserConn))
}

// 当用户连接
func (l *LiveRoom) handleUserConn(c *websocket.Conn) {
	if l.currentUser != nil {
		// 关闭上一个用户，仅允许一个用户读取弹幕
		_ = l.currentUser.Close()
		if l.lastUserCancel != nil {
			l.lastUserCancel()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	l.lastUserCancel = cancel
	l.currentUser = c

	done := ctx.Done()
	for {
		select {
		case <-done:
			break
		case data := <-l.DMChan:
			_ = c.WriteMessage(websocket.TextMessage, data)
		}
	}
}

// Close 关闭直播间监听
func (l *LiveRoom) Close() error {
	if l.heartbeatTK != nil {
		l.heartbeatTK.Stop()
		_ = l.sdk.AppEnd(l.GameID)
	}
	if l.client != nil {
		_ = l.client.Close()
	}
	if l.currentUser != nil {
		_ = l.currentUser.Close()
	}
	return nil
}

// 收到并处理弹幕消息
func (l *LiveRoom) handleDM(msg *proto.Message) error {
	// 单条消息raw 如果需要自己解析可以使用
	data := msg.Payload()
	log.Println("receive dm:", string(data))
	for {
		select {
		case l.DMChan <- data:
			return nil
		default:
			// 队列满了，丢弃
			old := <-l.DMChan
			log.Println("dm queue full, discard old:", string(old))
		}
	}
}

// 心跳精灵
func (l *LiveRoom) heartbeatDaemon() {
	// 启用项目心跳 20s一次
	// see https://open-live.bilibili.com/document/eba8e2e1-847d-e908-2e5c-7a1ec7d9266f
	for {
		select {
		case <-l.heartbeatTK.C:
			// 心跳
			if err := l.sdk.AppHeartbeat(l.GameID); err != nil {
				log.Println("Heartbeat fail", err)
			}
		}
	}
}
