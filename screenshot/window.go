package screenshot

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gitee.com/andrewgithub/FireShotGo/firetheme"
	"gitee.com/andrewgithub/FireShotGo/resources"
	"github.com/golang/glog"
	"image/color"
	"strconv"
)

func (fs *FireShotGO) BuildEditWindow() {
	// 设置主题支持中文
	fs.App.Settings().SetTheme(&firetheme.ShanGShouJianSongTheme{RefThemeApp: fs.App, FireFontSizeName: FireShotFontSize})
	// 创建主窗口
	fs.Win = fs.App.NewWindow(fmt.Sprintf("FireShotGO: screenshot @ %s", fs.ScreenshotTime.Format("2006-01-02 15:04:05")))
	// 设置工具图标
	fs.Win.SetIcon(resources.GoShotIconPng)

	// 创建菜单栏
	fs.Win.SetMainMenu(fs.MakeFireShotMenu())

	// 截屏图片操作视窗.
	fs.viewPort = NewViewPort(fs)

	// Side toolbar.
	cropTopLeft := widget.NewButtonWithIcon("", resources.CropTopLeft,
		func() {
			fs.status.SetText("点击左裁剪的上角")
			fs.viewPort.SetOp(CropTopLeft)
		})
	cropBottomRight := widget.NewButtonWithIcon("", resources.CropBottomRight,
		func() {
			fs.status.SetText("点击裁剪的左下角")
			fs.viewPort.SetOp(CropBottomRight)
		})
	cropReset := widget.NewButtonWithIcon("", resources.Reset, func() {
		fs.viewPort.cropReset()
		fs.viewPort.SetOp(NoOp)
	})

	circleButton := widget.NewButton("圆 (alt+c)", func() { fs.viewPort.SetOp(DrawCircle) })
	circleButton.SetIcon(resources.DrawCircle)

	fs.thicknessEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	fs.thicknessEntry.SetPlaceHolder(fmt.Sprintf("%g", fs.viewPort.Thickness))
	fs.thicknessEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Thickness changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			fs.viewPort.Thickness = val
			fs.App.Preferences().SetFloat(ThicknessPreference, val)
		}
	}

	fs.colorSample = canvas.NewRectangle(fs.viewPort.DrawingColor)
	size1d := theme.IconInlineSize()
	size := fyne.NewSize(5*size1d, size1d)
	fs.colorSample.SetMinSize(size)
	fs.colorSample.Resize(size)

	fs.miniMap = NewMiniMap(fs, fs.viewPort)

	toolBar := container.NewVBox(
		fs.miniMap,
		widget.NewButtonWithIcon("箭头 (alt+a)", resources.DrawArrow,
			func() { fs.viewPort.SetOp(DrawArrow) }),
		// FIXME: 已添加矢量图标 2021-09-30
		widget.NewButtonWithIcon("直线 (alt+l)", resources.DrawLine,
			func() { fs.viewPort.SetOp(DrawStraightLine) }),
		widget.NewButtonWithIcon("虚线 (alt+d)", resources.DrawDottedLine,
			func() { fs.viewPort.SetOp(DrawDottedLine) }),
		circleButton,
		container.NewHBox(
			widget.NewLabel("裁剪:"),
			cropTopLeft,
			cropBottomRight,
			cropReset,
		),
		container.NewHBox(
			widget.NewIcon(resources.Thickness), fs.thicknessEntry,
			widget.NewButtonWithIcon("", resources.ColorWheel, func() { fs.colorPicker() }),
			fs.colorSample,
		),
		widget.NewButtonWithIcon("文本 (alt+t)", resources.DrawText,
			func() { fs.viewPort.SetOp(DrawText) }),
	)

	// Status bar with zoom control.
	fs.zoomEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	fs.zoomEntry.SetPlaceHolder("0.0")
	fs.zoomEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Zoom level changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			fs.viewPort.Log2Zoom = val
			fs.viewPort.updateViewSize()
			fs.viewPort.Refresh()
		}
	}
	zoomReset := widget.NewButton("", func() {
		fs.zoomEntry.SetText("0")
		fs.viewPort.Log2Zoom = 0
		fs.viewPort.updateViewSize()
		fs.viewPort.Refresh()
	})
	zoomReset.SetIcon(resources.Reset)
	fs.status = widget.NewLabel(fmt.Sprintf("Image size: %s", fs.Screenshot.Bounds()))

	statusBar := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(widget.NewLabel("Zoom:"), fs.zoomEntry, zoomReset),
		fs.status,
	)

	// Stitch all together.
	split := container.NewHSplit(
		toolBar,
		fs.viewPort,
	)
	split.Offset = 0.2

	topLevel := container.NewBorder(
		nil, statusBar, nil, nil, container.NewMax(split))
	fs.Win.SetContent(topLevel)
	fs.Win.Resize(fyne.NewSize(1024.0, 768.0))

	// Register shortcuts.
	fs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: desktop.ControlModifier},
		func(shortcut fyne.Shortcut) {
			glog.Infof("Quit requested by shortcut %s", shortcut.ShortcutName())
			fs.App.Quit()
		})

	fs.RegisterShortcuts()
}

func (fs *FireShotGO) colorPicker() {
	glog.V(2).Infof("colorPicker():")
	picker := dialog.NewColorPicker(
		"Pick a Color", "Select color for edits",
		func(c color.Color) {
			fs.viewPort.DrawingColor = c
			fs.SetColorPreference(DrawingColorPreference, c)
			fs.colorSample.FillColor = c
			fs.colorSample.Refresh()
		},
		fs.Win)
	picker.Show()
}

// MakeFireShotMenu 创建FireShotGo菜单栏
func (fs *FireShotGO) MakeFireShotMenu() *fyne.MainMenu {
	// 构建文件菜单
	menuFile := fyne.NewMenu("文件",
		fyne.NewMenuItem("保存 (ctrl+s)", func() { fs.SaveImage() }),
		fyne.NewMenuItem("截屏", func() { fs.DelayedScreenshotForm() }),
	) // Quit is added automatically.

	// 构建编辑菜单
	menuSet := fyne.NewMenu("编辑",
		fyne.NewMenuItem("字体大小",
			func() {
				fs.fireShotGoFont.FireShotFontEdit(fs)
			}),
		fyne.NewMenuItem("复制 (ctrl+c)", func() { fs.CopyImageToClipboard() }),
	)

	// 构建云存储菜单
	menuShare := fyne.NewMenu("云存储",
		fyne.NewMenuItem("谷歌云 (ctrl+g)", func() { fs.ShareWithGoogleDrive() }),
		fyne.NewMenuItem("七牛云 ", func() { fs.ShareWithQiNiuDrive() }),
	)

	// 构建帮助菜单
	menuHelp := fyne.NewMenu("帮助",
		fyne.NewMenuItem("快捷方式 (ctrl+?)", func() { fs.ShowShortcutsPage() }),
		fyne.NewMenuItem("联系我们 ", func() { fs.ConnectUsPage() }),
	)
	return fyne.NewMainMenu(menuFile, menuSet, menuShare, menuHelp)

}
