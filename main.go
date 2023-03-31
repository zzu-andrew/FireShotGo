package main

import (
	"flag"
	"gitee.com/andrewgithub/FireShotGo/screenshot"
	"github.com/golang/glog"
)

/**
 * @brief 使用fyne实现截图跨平台截图工具
 */

// 后期仅支持fyne库版本的截图功能
func main() {
	// 参数解析
	flag.Parse()
	defer glog.Flush()

	// 开启截屏软件主程序
	screenshot.Run()
}
