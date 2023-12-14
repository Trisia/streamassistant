package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/go-vgo/robotgo"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/valyala/fasthttp"
	"io"
	"log"
	"net/url"
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

func main() {
	flag.Parse()
	if *verFlag {
		fmt.Printf("\nStrean Assustant Version: %s\n", Version)
		return
	}
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
	app.Use(cors.New())
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

	// 注册快捷键
	shortKey := InitShortKey()
	shortKey.Register(app)

	// 注册直播间
	live := InitLive()
	live.Register(app)
	defer live.Close()

	httpClient := &fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	}

	// 下载图片
	app.Get("icon", func(ctx *fiber.Ctx) error {
		path := ctx.Query("path")
		src, _ := url.QueryUnescape(path)
		if src == "" {
			return ctx.SendStatus(404)
		}
		code, body, err := httpClient.Get(nil, src)
		if err != nil {
			return err
		}
		if code != 200 {
			return ctx.SendStatus(code)
		}
		_, err = ctx.Write(body)
		return err
	})

	addr := ":80"
	log.Printf("Stream Assistant V%s Start at http://127.0.0.1%s\n", Version, addr)
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
