package main

import (
	"flag"
	"gitee.com/andrewgithub/FireShotGo/screenshot"
)

/**
 * @brief 使用fyne实现截图跨平台截图工具
 */

// 后期仅支持fyne库版本的截图功能
func main() {
	// 参数解析
	flag.Parse()

	screenshot.Run()
}
