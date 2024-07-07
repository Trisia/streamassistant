package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/go-vgo/robotgo"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/valyala/fasthttp"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	Version   = "1.1.2"
	ImagePath = "截图"
	LogPaths  = "logs"
)

// Config 配置
type Config struct {
	AccessKey       string // access_key
	AccessKeySecret string // access_key_secret
	IdCode          string //  主播身份码
	AppID           int64  // 应用id
}

// Cfg 配置实体
var Cfg = &Config{}

var (
	micSwitch          = true
	recordSwitch       = false
	streamSwitch       = false
	streamAt     int64 = 0 // 直播开始时间(Unix毫秒)，0 为未开始
)

//go:embed static
var viewsFS embed.FS

//var verFlag = flag.Bool("v", false, "show version")

func main() {
	fmt.Printf("Strean Assustant Version: %s\n\n", Version)
	//flag.Parse()
	//if *verFlag {
	//	return
	//}

	// 判断 os.Args 是否有 code= 或 -code= 参数
	for i := range os.Args {
		if strings.Contains(os.Args[i], "code=") {
			offset := strings.Index(os.Args[i], "code=")
			Cfg.IdCode = os.Args[i][offset+5:]
			break
		}
	}

	cfgBin, _ := os.ReadFile("config.json")
	if len(cfgBin) > 0 {
		err := json.Unmarshal(cfgBin, Cfg)
		if err != nil {
			log.Fatal("读取配置文件失败", err)
		}
	} else {
		log.Fatal("未找到配置文件")
	}

	if Cfg.IdCode == "" {
		log.Println("请在启动参数或配置文件中指定主播身份码 start.exe code=XXX")
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
	app.Use("/static", filesystem.New(filesystem.Config{
		Root:       http.FS(viewsFS),
		PathPrefix: "static",
	}))
	//app.Static("/static", "./static")
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

	addr := ":30080"
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("请通过手机浏览器访问下面地址:")
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ipnet.IP.To4() != nil && ipnet.IP.String() != "" {
				if strings.HasPrefix(ipnet.IP.String(), "169.") || strings.HasPrefix(ipnet.IP.String(), "127.") {
					continue
				}
				log.Printf("\t\thttp://%s%s\n", ipnet.IP.String(), addr)
			}
		}
	}
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
