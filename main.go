package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/go-vgo/robotgo"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/vtb-link/bianka/live"
	"github.com/vtb-link/bianka/proto"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

const Version = "1.0.1"

var ImagePath = "截图"
var LogPaths = "logs"

var (
	micSwitch          = true
	recordSwitch       = false
	streamSwitch       = false
	streamAt     int64 = 0 // 直播开始时间(Unix毫秒)，0 为未开始
)

//go:embed static
var viewsFS embed.FS

var verFlag = flag.Bool("v", false, "show version")

// 当前用户
var currentUser *websocket.Conn
var dmChan = make(chan []byte, 32)

func main() {
	flag.Parse()
	if *verFlag {
		fmt.Printf("\nStrean Assustant Version: %s\n", Version)
		return
	}

	sdkConfig := live.NewConfig(AccessKey, AccessKeySecret, int64(AppId))
	// 创建sdk实例
	sdk := live.NewClient(sdkConfig)
	// app start
	startResp, err := sdk.AppStart(IdCode)
	if err != nil {
		log.Fatal(err)
	}
	tk := heartbeatDaemon(sdk, startResp)
	defer func() {
		tk.Stop()
		_ = sdk.AppEnd(startResp.GameInfo.GameID)
	}()

	// 一键开启websocket
	wsClient, err := sdk.StartWebsocket(startResp, map[uint32]live.DispatcherHandle{proto.OperationMessage: handleDM}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer wsClient.Close()

	_ = os.Mkdir(ImagePath, os.ModePerm)
	_ = os.Mkdir(LogPaths, os.ModePerm)
	logger := log.Default()
	logWriter, err := os.Create(filepath.Join(LogPaths, "streamassistant.log"))
	if err != nil {
		log.Fatal(err)
	}
	defer logWriter.Close()
	logger.SetOutput(io.MultiWriter(logWriter, os.Stdout))

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	//app.Use("/static", filesystem.New(filesystem.Config{
	//	Root:       http.FS(viewsFS),
	//	PathPrefix: "static",
	//}))
	app.Static("/static", "./static")
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect(fmt.Sprintf("/static/index.html?ts=%d", time.Now().Unix()))
	})
	// 查询开关状态
	app.Get("/switch-state", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"micSwitch":    micSwitch,
			"recordSwitch": recordSwitch,
			"streamSwitch": streamSwitch,
			"streamAt":     streamAt,
		})
	})

	app.Get("/capture-screen", func(c *fiber.Ctx) error {
		//time.Sleep(time.Second * 3)
		go captureScreen()
		return nil
	})
	// 开始录制 开关
	app.Get("/record-switch", func(c *fiber.Ctx) error {
		_ = robotgo.KeyTap("f7")
		recordSwitch = !recordSwitch
		//time.Sleep(time.Second * 3)
		if recordSwitch {
			log.Println("record switch to: ON")
		} else {
			log.Println("record switch to: OFF")
		}
		return c.JSON(recordSwitch)
	})
	// 直播 开关
	app.Get("/stream-switch", func(c *fiber.Ctx) error {
		_ = robotgo.KeyTap("f8")
		streamSwitch = !streamSwitch
		recordSwitch = streamSwitch
		//time.Sleep(time.Second * 3)
		if streamSwitch {
			log.Println("stream switch to: ON")
			streamAt = time.Now().UnixMilli()
		} else {
			streamAt = 0
			log.Println("stream switch to: OFF")
		}
		return c.JSON(streamSwitch)
	})
	// 麦克风 开关
	app.Get("/mic-switch", func(c *fiber.Ctx) error {
		_ = robotgo.KeyTap("f9")
		micSwitch = !micSwitch
		//time.Sleep(time.Second * 3)
		if micSwitch {
			log.Println("mic switch to: ON")
		} else {
			log.Println("mic switch to: OFF")
		}

		return c.JSON(micSwitch)
	})

	// 弹幕消息通道
	app.Get("/dm", websocket.New(func(c *websocket.Conn) {
		if currentUser != nil {
			// 关闭上一个用户，仅允许一个用户读取弹幕
			_ = currentUser.Close()
		}
		for {
			data := <-dmChan
			_ = c.WriteMessage(websocket.TextMessage, data)
		}
	}))

	addr := ":80"
	log.Printf("Stream Assistant V%s Start at http://127.0.0.1%s\n", Version, addr)
	_ = app.Listen(addr)
}

// 收到并处理弹幕消息
func handleDM(msg *proto.Message) error {
	// 单条消息raw 如果需要自己解析可以使用
	data := msg.Payload()
	log.Println(string(data))
	for {
		select {
		case dmChan <- data:
			return nil
		default:
			// 队列满了，丢弃
			old := <-dmChan
			log.Println("dm queue full, discard old:", string(old))
		}
	}
}

// 心跳精灵
func heartbeatDaemon(sdk *live.Client, startResp *live.AppStartResponse) *time.Ticker {
	// 启用项目心跳 20s一次
	// see https://open-live.bilibili.com/document/eba8e2e1-847d-e908-2e5c-7a1ec7d9266f
	tk := time.NewTicker(time.Second * 20)
	go func() {
		for {
			select {
			case <-tk.C:
				// 心跳
				if err := sdk.AppHeartbeat(startResp.GameInfo.GameID); err != nil {
					log.Println("Heartbeat fail", err)
				}
			}
		}
	}()
	return tk
}

// 截图
func captureScreen() {
	num := robotgo.DisplaysNum()
	i := num - 1
	if i < 0 {
		return
	}
	x, y, w, h := robotgo.GetDisplayBounds(i)
	robotgo.DisplayID = i
	filePath := filepath.Join(ImagePath, fmt.Sprintf("%s.jpeg", time.Now().Format("20060102150405")))
	img1 := robotgo.CaptureImg(x, y, w, h)

	_ = robotgo.SaveJpeg(img1, filePath, 80)
	log.Printf("capture screen save at: %s\n", filePath)
}
