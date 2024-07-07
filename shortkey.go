package main

import (
	"github.com/go-vgo/robotgo"
	"github.com/gofiber/fiber/v2"
	"log"
	"time"
)

// ShortKeyHandler 快捷键处理器
type ShortKeyHandler struct {
}

func InitShortKey() *ShortKeyHandler {
	return &ShortKeyHandler{}
}

// Register 注册快捷键
func (s *ShortKeyHandler) Register(app *fiber.App) {
	skh := ShortKeyHandler{}
	// 截图
	app.Get("/capture-screen", skh.handleCaptureScreen)
	// 开始录制 开关
	app.Get("/record-switch", skh.handleRecordSwitch)
	// 直播 开关
	app.Get("/stream-switch", skh.handleStreamSwitch)
	// 麦克风 开关
	app.Get("/mic-switch", skh.handleMicSwitch)

	// Windows键
	app.Get("/win", skh.handleWindows)
	// 切换窗口
	app.Get("/tab-switch", skh.handleTabSwitch)
}

// handleCaptureScreen 截图
func (s *ShortKeyHandler) handleCaptureScreen(c *fiber.Ctx) error {
	//time.Sleep(time.Second * 3)
	go captureScreen()
	return nil
}

// handleRecordSwitch  开始录制 开关
func (s *ShortKeyHandler) handleRecordSwitch(c *fiber.Ctx) error {
	_ = robotgo.KeyTap("f7")
	recordSwitch = !recordSwitch
	//time.Sleep(time.Second * 3)
	if recordSwitch {
		log.Println("record switch to: ON")
	} else {
		log.Println("record switch to: OFF")
	}
	return c.JSON(recordSwitch)
}

// handleStreamSwitch 直播 开关
func (s *ShortKeyHandler) handleStreamSwitch(c *fiber.Ctx) error {
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
}

// handleMicSwitch 麦克风 开关
func (s *ShortKeyHandler) handleMicSwitch(c *fiber.Ctx) error {
	_ = robotgo.KeyTap("f9")
	micSwitch = !micSwitch
	//time.Sleep(time.Second * 3)
	if micSwitch {
		log.Println("mic switch to: ON")
	} else {
		log.Println("mic switch to: OFF")
	}
	return c.JSON(micSwitch)
}

// 按下windows键
func (s *ShortKeyHandler) handleWindows(ctx *fiber.Ctx) error {
	_ = robotgo.KeyTap("cmd")
	return nil
}

// 切换Tab
func (s *ShortKeyHandler) handleTabSwitch(ctx *fiber.Ctx) error {
	_ = robotgo.KeyTap("tab", "alt")
	return nil
}
