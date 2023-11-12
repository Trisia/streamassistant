package main

import (
	"embed"
	"fmt"
	"github.com/go-vgo/robotgo"
	"github.com/gofiber/fiber/v2"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

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

func main() {
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
	addr := ":80"
	log.Printf("Stream Assistant Start at http://127.0.0.1%s\n", addr)
	_ = app.Listen(addr)
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
