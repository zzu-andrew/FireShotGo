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

func (gs *FireShotGO) BuildEditWindow() {
	// 设置主题支持中文
	gs.App.Settings().SetTheme(&firetheme.ShanGShouJianSongTheme{RefThemeApp: gs.App, FireFontSizeName: FireShotFontSize})
	// 创建主窗口
	gs.Win = gs.App.NewWindow(fmt.Sprintf("FireShotGO: screenshot @ %s", gs.ScreenshotTime.Format("2006-01-02 15:04:05")))
	// 设置工具图标
	gs.Win.SetIcon(resources.GoShotIconPng)

	// 创建菜单栏
	gs.Win.SetMainMenu(gs.MakeFireShotMenu())

	// 截屏图片操作视窗.
	gs.viewPort = NewViewPort(gs)

	// Side toolbar.
	cropTopLeft := widget.NewButtonWithIcon("", resources.CropTopLeft,
		func() {
			gs.status.SetText("点击左裁剪的上角")
			gs.viewPort.SetOp(CropTopLeft)
		})
	cropBottomRight := widget.NewButtonWithIcon("", resources.CropBottomRight,
		func() {
			gs.status.SetText("点击裁剪的左下角")
			gs.viewPort.SetOp(CropBottomRight)
		})
	cropReset := widget.NewButtonWithIcon("", resources.Reset, func() {
		gs.viewPort.cropReset()
		gs.viewPort.SetOp(NoOp)
	})

	circleButton := widget.NewButton("圆 (alt+c)", func() { gs.viewPort.SetOp(DrawCircle) })
	circleButton.SetIcon(resources.DrawCircle)

	gs.thicknessEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	gs.thicknessEntry.SetPlaceHolder(fmt.Sprintf("%g", gs.viewPort.Thickness))
	gs.thicknessEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Thickness changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			gs.viewPort.Thickness = val
			gs.App.Preferences().SetFloat(ThicknessPreference, val)
		}
	}

	gs.colorSample = canvas.NewRectangle(gs.viewPort.DrawingColor)
	size1d := theme.IconInlineSize()
	size := fyne.NewSize(5*size1d, size1d)
	gs.colorSample.SetMinSize(size)
	gs.colorSample.Resize(size)

	gs.miniMap = NewMiniMap(gs, gs.viewPort)

	toolBar := container.NewVBox(
		gs.miniMap,
		widget.NewButtonWithIcon("箭头 (alt+a)", resources.DrawArrow,
			func() { gs.viewPort.SetOp(DrawArrow) }),
		// FIXME: 已添加矢量图标 2021-09-30
		widget.NewButtonWithIcon("直线 (alt+l)", resources.DrawLine,
			func() { gs.viewPort.SetOp(DrawStraightLine) }),
		circleButton,
		container.NewHBox(
			widget.NewLabel("裁剪:"),
			cropTopLeft,
			cropBottomRight,
			cropReset,
		),
		container.NewHBox(
			widget.NewIcon(resources.Thickness), gs.thicknessEntry,
			widget.NewButtonWithIcon("", resources.ColorWheel, func() { gs.colorPicker() }),
			gs.colorSample,
		),
		widget.NewButtonWithIcon("文本 (alt+t)", resources.DrawText,
			func() { gs.viewPort.SetOp(DrawText) }),
	)

	// Status bar with zoom control.
	gs.zoomEntry = &widget.Entry{Validator: validation.NewRegexp(`\d`, "Must contain a number")}
	gs.zoomEntry.SetPlaceHolder("0.0")
	gs.zoomEntry.OnChanged = func(str string) {
		glog.V(2).Infof("Zoom level changed to %s", str)
		val, err := strconv.ParseFloat(str, 64)
		if err == nil {
			gs.viewPort.Log2Zoom = val
			gs.viewPort.updateViewSize()
			gs.viewPort.Refresh()
		}
	}
	zoomReset := widget.NewButton("", func() {
		gs.zoomEntry.SetText("0")
		gs.viewPort.Log2Zoom = 0
		gs.viewPort.updateViewSize()
		gs.viewPort.Refresh()
	})
	zoomReset.SetIcon(resources.Reset)
	gs.status = widget.NewLabel(fmt.Sprintf("Image size: %s", gs.Screenshot.Bounds()))

	statusBar := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(widget.NewLabel("Zoom:"), gs.zoomEntry, zoomReset),
		gs.status,
	)

	// Stitch all together.
	split := container.NewHSplit(
		toolBar,
		gs.viewPort,
	)
	split.Offset = 0.2

	topLevel := container.NewBorder(
		nil, statusBar, nil, nil, container.NewMax(split))
	gs.Win.SetContent(topLevel)
	gs.Win.Resize(fyne.NewSize(1024.0, 768.0))

	// Register shortcuts.
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: desktop.ControlModifier},
		func(shortcut fyne.Shortcut) {
			glog.Infof("Quit requested by shortcut %s", shortcut.ShortcutName())
			gs.App.Quit()
		})

	gs.RegisterShortcuts()
}

func (gs *FireShotGO) colorPicker() {
	glog.V(2).Infof("colorPicker():")
	picker := dialog.NewColorPicker(
		"Pick a Color", "Select color for edits",
		func(c color.Color) {
			gs.viewPort.DrawingColor = c
			gs.SetColorPreference(DrawingColorPreference, c)
			gs.colorSample.FillColor = c
			gs.colorSample.Refresh()
		},
		gs.Win)
	picker.Show()
}

// MakeFireShotMenu 创建FireShotGo菜单栏
func (gs *FireShotGO) MakeFireShotMenu() *fyne.MainMenu {
	// 构建文件菜单
	menuFile := fyne.NewMenu("文件",
		fyne.NewMenuItem("保存 (ctrl+s)", func() { gs.SaveImage() }),
		fyne.NewMenuItem("截屏", func() { gs.DelayedScreenshotForm() }),
	) // Quit is added automatically.

	// 构建编辑菜单
	menuSet := fyne.NewMenu("编辑",
		fyne.NewMenuItem("字体大小",
			func() {
				gs.fireShotGoFont.FireShotFontEdit(gs)
			}),
		fyne.NewMenuItem("复制 (ctrl+c)", func() { gs.CopyImageToClipboard() }),
	)

	// 构建云存储菜单
	menuShare := fyne.NewMenu("云存储",
		fyne.NewMenuItem("谷歌云 (ctrl+g)", func() { gs.ShareWithGoogleDrive() }),
		fyne.NewMenuItem("七牛云 ", func() { gs.ShareWithQiNiuDrive() }),
	)

	// 构建帮助菜单
	menuHelp := fyne.NewMenu("帮助",
		fyne.NewMenuItem("快捷方式 (ctrl+?)", func() { gs.ShowShortcutsPage() }),
		fyne.NewMenuItem("联系我们 ", func() { gs.ConnectUsPage() }),
	)
	return fyne.NewMainMenu(menuFile, menuSet, menuShare, menuHelp)

}
