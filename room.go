package main

import (
	"context"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/vtb-link/bianka/basic"
	"github.com/vtb-link/bianka/live"
	"github.com/vtb-link/bianka/proto"
	"log"
	"time"
)

// InitLive 初始化直播
func InitLive() *LiveRoom {
	room := &LiveRoom{
		DMChan:      make(chan []byte, 128),
		heartbeatTK: time.NewTicker(time.Second * 20),
	}
	sdkConfig := live.NewConfig(Cfg.AccessKey, Cfg.AccessKeySecret, int64(Cfg.AppID))
	// 创建sdk实例
	room.sdk = live.NewClient(sdkConfig)
	// app start
	startResp, err := room.sdk.AppStart(Cfg.IdCode)
	if err != nil {
		log.Fatal(err)
	}
	room.GameID = startResp.GameInfo.GameID

	go room.heartbeatDaemon()

	// 开启与直播间的连接，并设置消息处理器
	room.client, err = basic.StartWebsocket(startResp, basic.DispatcherHandleMap{
		proto.OperationMessage: room.handleDM,
	}, nil, basic.DefaultLoggerGenerator())
	//room.client, err = room.sdk.StartWebsocket(startResp, map[uint32]live.DispatcherHandle{proto.OperationMessage: room.handleDM}, nil)
	if err != nil {
		log.Fatal(err)
	}

	return room
}

// LiveRoom 直播房间
type LiveRoom struct {
	sdk            *live.Client
	client         *basic.WsClient
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
func (l *LiveRoom) handleDM(wsClient *basic.WsClient, msg *proto.Message) error {
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
