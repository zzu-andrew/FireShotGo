// Package screenshot implements the screenshot edit window.
//
// It's the main part of the application: it may be run after a
// fork(), if the main program was started as a system tray app.
package screenshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"gitee.com/andrewgithub/FireShotGo/clipboard"
	"gitee.com/andrewgithub/FireShotGo/cloud"
	"gitee.com/andrewgithub/FireShotGo/resources"
	"github.com/golang/glog"
	"github.com/kbinani/screenshot"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path"
	"strconv"
	"time"
)

type FireShotGO struct {
	// 应用和窗口
	App fyne.App
	// 主窗口
	Win fyne.Window
	// topLevel 备份top层的窗口，用于更改主题一类的刷新使用
	TopLevelContainer fyne.Container

	// 原始截图信息
	OriginalScreenshot *image.RGBA
	// 截图时间记录
	ScreenshotTime time.Time

	// 编辑之后的截图信息，每添加一个fileter这里都进行叠加一次
	Screenshot *image.RGBA // The edited/composed screenshot
	CropRect   image.Rectangle
	// 所有的fileter都添加到这里
	Filters []ImageFilter // Configured filters: each filter is one edition to the image.

	// UI 元素
	// zoomEntry 缩放窗口控件 thicknessEntry 设置线条粗细的控件
	zoomEntry, thicknessEntry *widget.Entry
	// colorSample 颜色示例窗口
	colorSample *canvas.Rectangle
	// 工具左下角显示当前工作状态
	status *widget.Label
	// 预览窗口
	viewPort       *ViewPort
	viewPortScroll *container.Scroll
	// 缩略窗口
	miniMap *MiniMap

	// 快捷键展示控件
	shortcutsDialog dialog.Dialog
	// 延时截取屏幕的时候掉用出来，其实是一个表单
	delayedScreenshotDialog dialog.Dialog
	// 联系我们界面
	connectUsDialog dialog.Dialog

	// 谷歌云盘
	gDrive          *cloud.Manager
	gDriveNumShared int

	// 七牛云
	qDrive          *cloud.QiNiuManager
	qDriveNumShared int
	// 七牛云需要支持同步和异步两种方式，这里需要拿到一起创建的七牛云的dialog
	qNiuDialog dialog.Dialog

	// 记录当前需要截取那个屏幕,默认情况下是0
	displayIndex int

	// 当前系统字体大小
	fireShotGoFont FireShotFont
}

type ImageFilter interface {
	// Apply filter, shifted (dx, dy) pixels -- e.g. if a filter draws a circle on
	// top of the image, it should add (dx, dy) to the circle center.
	Apply(image image.Image) image.Image
}

// ApplyFilters will apply `Filters` to the `CropRect` of the original image
// and regenerate Screenshot.
// If full == true, regenerates full Screenshot. If false, regenerates only
// visible area.
// 绘制预览图
func (fs *FireShotGO) ApplyFilters(full bool) {
	glog.V(2).Infof("ApplyFilters: %d filters", len(fs.Filters))
	// 图像叠加
	filteredImage := image.Image(fs.OriginalScreenshot)
	for _, filter := range fs.Filters {
		filteredImage = filter.Apply(filteredImage)
	}

	if fs.Screenshot == fs.OriginalScreenshot || fs.Screenshot.Rect.Dx() != fs.CropRect.Dx() || fs.Screenshot.Rect.Dy() != fs.CropRect.Dy() {
		// Recreate image buffer.
		crop := image.NewRGBA(image.Rect(0, 0, fs.CropRect.Dx(), fs.CropRect.Dy()))
		fs.Screenshot = crop
		full = true // Regenerate the full buffer.
	}
	if full {
		draw.Src.Draw(fs.Screenshot, fs.Screenshot.Rect, filteredImage, fs.CropRect.Min)
	} else {
		var tgtRect image.Rectangle
		tgtRect.Min = image.Point{X: fs.viewPort.viewX, Y: fs.viewPort.viewY}
		tgtRect.Max = tgtRect.Min.Add(image.Point{X: fs.viewPort.viewW, Y: fs.viewPort.viewH})
		srcPoint := fs.CropRect.Min.Add(tgtRect.Min)
		draw.Src.Draw(fs.Screenshot, tgtRect, filteredImage, srcPoint)
	}

	if fs.viewPort != nil {
		fs.viewPort.renderCache()
		fs.viewPort.Refresh()
	}
	if fs.miniMap != nil {
		fs.miniMap.renderCache()
		fs.miniMap.Refresh()
	}
}

// Run 截图的主程序
func Run() {
	// fyne 功能, 对Fyne不太了解的可以参考 https://gitee.com/andrewgithub/fyne-club
	// 里面有详细的go Fyne教程，并且每小节我都实现了对应的源码
	fireShotGo := &FireShotGO{
		// 使用给后期需要 独立配置参数的 Fyne需要使用  NewWithID 没有要求的可以使用app.New()
		// 使用带有ID的new方便后期绑定应用全局数据
		App: app.NewWithID("FireShotGo"),
	}
	// 开始截屏 --
	err := fireShotGo.MakeScreenshot()
	if err != nil {
		glog.Fatalf("Failed to capture screenshot: %s", err)
	}
	// 这里开始构建应用窗口
	fireShotGo.BuildEditWindow()
	// 开始运行主窗口
	fireShotGo.Win.ShowAndRun()
	fireShotGo.miniMap.updateViewPortRect()
	fireShotGo.miniMap.Refresh()
}

// MakeScreenshot 开始截屏
func (gs *FireShotGO) MakeScreenshot() error {

	n := screenshot.NumActiveDisplays()
	if n != 1 {
		// 已经支持多屏幕截图，这里给出屏幕个数
		glog.Warningf("检测到用户屏幕个数: %d，请在文件->截屏中配置需要截屏的序号", n)
	}

	if gs.displayIndex < 0 || gs.displayIndex > n {
		glog.Fatalf("displayIndex 非法请确认")
	}
	// 获取当前显示器左上角和右下角的位置信息 eg (0,0) (1920, 1080)
	bounds := screenshot.GetDisplayBounds(gs.displayIndex)

	glog.Infof("截图位置:(%d, %d) -> (%d, %d)",
		bounds.Min.X, bounds.Min.Y,
		bounds.Max.X, bounds.Max.Y)

	var err error
	// 根据指定的bounds信息截取屏幕
	gs.Screenshot, err = screenshot.CaptureRect(bounds)
	if err != nil {
		glog.Errorf("CaptureRect failed.")
		return err
	}
	// 将刚截好图的信息被分到原始截图信息上，以便后期使用
	gs.OriginalScreenshot = gs.Screenshot
	gs.ScreenshotTime = time.Now()
	gs.CropRect = gs.Screenshot.Bounds()

	glog.V(2).Infof("截屏边界: %+v\n", bounds)
	return nil
}

// UndoLastFilter cancels the last filter applied, and regenerates everything.
func (gs *FireShotGO) UndoLastFilter() {
	if len(gs.Filters) > 0 {
		gs.Filters = gs.Filters[:len(gs.Filters)-1]
		gs.ApplyFilters(true)
	}
}

// DefaultName returns a default name to the screenshot, based on date/time it was made.
func (gs *FireShotGO) DefaultName() string {
	return fmt.Sprintf("Screenshot %s",
		gs.ScreenshotTime.Format("2006-01-02 15-04-02"))
}
func (gs *FireShotGO) FireShotNameByTime() string {
	timestamp := time.Now().Unix()
	tm := time.Unix(timestamp, 0)

	return tm.Format("2006-01-02 03:04:05 PM")
}

// GetColorPreference 颜色信息设置，若是环境变量中没有设置就使用默认值
func (gs *FireShotGO) GetColorPreference(key string, defaultColor color.RGBA) color.RGBA {
	isSet := gs.App.Preferences().Bool(key)
	if !isSet {
		return defaultColor
	}
	r := gs.App.Preferences().Int(key + "_R")
	g := gs.App.Preferences().Int(key + "_G")
	b := gs.App.Preferences().Int(key + "_B")
	a := gs.App.Preferences().Int(key + "_A")
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

// SetColorPreference sets the given color in the given preferences key.
func (gs *FireShotGO) SetColorPreference(key string, c color.Color) {
	r, g, b, a := c.RGBA()
	gs.App.Preferences().SetInt(key+"_R", int(r))
	gs.App.Preferences().SetInt(key+"_G", int(g))
	gs.App.Preferences().SetInt(key+"_B", int(b))
	gs.App.Preferences().SetInt(key+"_A", int(a))
	gs.App.Preferences().SetBool(key, true)
}

const DefaultPathPreference = "DefaultPath"

// SaveImage opens a file save dialog box to save the currently edited screenshot.
func (gs *FireShotGO) SaveImage() {
	glog.V(2).Info("FireShotGO.SaveImage")
	var fileSave *dialog.FileDialog
	fileSave = dialog.NewFileSave(
		func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				glog.Errorf("Failed to save image: %s", err)
				gs.status.SetText(fmt.Sprintf("Failed to save image: %s", err))
				return
			}
			if writer == nil {
				gs.status.SetText("Save file cancelled.")
				return
			}
			glog.V(2).Infof("SaveImage(): URI=%s", writer.URI())
			defer func() { _ = writer.Close() }()

			// Always default to previous path used:
			defaultPath := path.Dir(writer.URI().Path())
			gs.App.Preferences().SetString(DefaultPathPreference, defaultPath)

			var contentBuffer bytes.Buffer
			_ = png.Encode(&contentBuffer, gs.Screenshot)
			content := contentBuffer.Bytes()
			_, err = writer.Write(content)
			if err != nil {
				glog.Errorf("Failed to save image to %q: %s", writer.URI(), fileSave)
				gs.status.SetText(fmt.Sprintf("Failed to save image to %q: %s", writer.URI(), err))
				return
			}
			gs.status.SetText(fmt.Sprintf("Saved image to %q", writer.URI()))
		}, gs.Win)
	fileSave.SetFileName(gs.DefaultName() + ".png")
	if defaultPath := gs.App.Preferences().String(DefaultPathPreference); defaultPath != "" {
		lister, err := storage.ListerForURI(storage.NewFileURI(defaultPath))
		if err == nil {
			fileSave.SetLocation(lister)
		} else {
			glog.Warningf("Cannot create a ListableURI for %q", defaultPath)
		}
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	fileSave.Resize(size)
	fileSave.Show()
}

// OpenImage opens a file save dialog box to save the currently edited screenshot.
func (gs *FireShotGO) OpenImage() {
	glog.V(2).Info("FireShotGO.OpenImage")
	var fileOpen *dialog.FileDialog
	fileOpen = dialog.NewFileOpen(
		func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				glog.Errorf("Failed to save image: %s", err)
				gs.status.SetText(fmt.Sprintf("Failed to save image: %s", err))
				return
			}
			if reader == nil {
				gs.status.SetText("Save file cancelled.")
				return
			}
			glog.V(2).Infof("SaveImage(): URI=%s", reader.URI())
			defer func() { _ = reader.Close() }()

			// Always default to previous path used:
			defaultPath := path.Dir(reader.URI().Path())
			gs.App.Preferences().SetString(DefaultPathPreference, defaultPath)

			var contentBuffer bytes.Buffer
			_ = png.Encode(&contentBuffer, gs.Screenshot)
			content := contentBuffer.Bytes()
			_, err = reader.Read(content)
			if err != nil {
				glog.Errorf("Failed to save image to %q: %s", reader.URI(), fileOpen)
				gs.status.SetText(fmt.Sprintf("Failed to save image to %q: %s", reader.URI(), err))
				return
			}
			gs.status.SetText(fmt.Sprintf("Saved image to %q", reader.URI()))
		}, gs.Win)

	fileOpen.SetFileName(gs.DefaultName() + ".png")
	if defaultPath := gs.App.Preferences().String(DefaultPathPreference); defaultPath != "" {
		lister, err := storage.ListerForURI(storage.NewFileURI(defaultPath))
		if err == nil {
			fileOpen.SetLocation(lister)
		} else {
			glog.Warningf("Cannot create a ListableURI for %q", defaultPath)
		}
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	fileOpen.Resize(size)
	fileOpen.Show()
}

func (gs *FireShotGO) CopyImageToClipboard() {
	glog.V(2).Info("FireShotGO.CopyImageToClipboard")
	err := clipboard.CopyImage(gs.Screenshot)
	if err != nil {
		glog.Errorf("Failed to copy to clipboard: %s", err)
		gs.status.SetText(fmt.Sprintf("Failed to copy to clipboard: %s", err))
	} else {
		gs.status.SetText(fmt.Sprintf("Screenshot copied to clipboard"))
	}
}

func (gs *FireShotGO) MakeInputTab(_ fyne.Window) fyne.CanvasObject {
	selectEntry := widget.NewSelectEntry([]string{"Option A", "Option B", "Option C"})
	selectEntry.PlaceHolder = "Type or select"
	disabledCheck := widget.NewCheck("Disabled check", func(bool) {})
	disabledCheck.Disable()
	radio := widget.NewRadioGroup([]string{"Radio Item 1", "Radio Item 2"}, func(s string) { fmt.Println("selected", s) })
	radio.Horizontal = true
	disabledRadio := widget.NewRadioGroup([]string{"Disabled radio"}, func(string) {})
	disabledRadio.Disable()

	return container.NewVBox(
		widget.NewSelect([]string{"Option 1", "Option 2", "Option 3"}, func(s string) { fmt.Println("selected", s) }),
		selectEntry,
		widget.NewCheck("Check", func(on bool) { fmt.Println("checked", on) }),
		disabledCheck,
		radio,
		disabledRadio,
		widget.NewSlider(0, 100),
	)
}

const (
	GoogleDriveTokenPreference = "google_drive_token"
	QiNiuAccessKey             = "qiNiuAccessKey"
	QiNiuSecretKey             = "qiNiuSecretKey"
	QiNiuBucket                = "qiNiuBucket"
)

var (
	GoogleDrivePath = []string{"FireShotGO"}
)

func (gs *FireShotGO) ShareWithQiNiuDrive() {
	glog.V(2).Infof("FireShotGO.ShareWithQiNiuDrive")

	gs.status.SetText("开始连接七牛云盘 ...")
	// 采用异步上传方式进行图片上传
	// 获取用户信息
	accessEntry := widget.NewEntry()
	// 因为认证消息的字符可能性很多这里不进行检验
	accessEntry.Validator = func(text string) error {
		if len(text) > 100 {
			return errors.New("access text is too long")
		}
		return nil
	}
	// 获取系统默认变量
	qiNiuAccessConfig := gs.App.Preferences().String(QiNiuAccessKey)
	// 这里设置预写字段
	accessEntry.SetText(qiNiuAccessConfig)
	// 在写占位符，占位符使用历史记录的内容，用于提示用户怎样书写
	accessEntry.SetPlaceHolder(qiNiuAccessConfig)
	accessEntry.Resize(fyne.NewSize(400, 40))

	// 获取用户信息
	secretEntry := widget.NewEntry()
	// 因为认证消息的字符可能性很多这里不进行检验
	secretEntry.Validator = func(text string) error {
		if len(text) > 100 {
			return errors.New("access text is too long")
		}
		return nil
	}
	// 获取系统默认变量
	qiNiuSecretConfig := gs.App.Preferences().String(QiNiuSecretKey)
	// 这里设置预写字段
	secretEntry.SetText(qiNiuSecretConfig)
	// 在写占位符，占位符使用历史记录的内容，用于提示用户怎样书写
	secretEntry.SetPlaceHolder(qiNiuSecretConfig)
	secretEntry.Resize(fyne.NewSize(400, 40))

	// 获取用户信息
	bucketEntry := widget.NewEntry()
	// 因为认证消息的字符可能性很多这里不进行检验
	bucketEntry.Validator = func(text string) error {
		if len(text) > 100 {
			return errors.New("access text is too long")
		}
		return nil
	}
	// 获取系统默认变量
	qiNiuBucketConfig := gs.App.Preferences().String(QiNiuBucket)
	// 这里设置预写字段
	bucketEntry.SetText(qiNiuBucketConfig)
	// 在写占位符，占位符使用历史记录的内容，用于提示用户怎样书写
	bucketEntry.SetPlaceHolder(qiNiuBucketConfig)
	bucketEntry.Resize(fyne.NewSize(400, 40))

	if gs.qDrive == nil {
		gs.qDrive, _ = cloud.NewQiNiu("", "", "")
	}

	if gs.qNiuDialog == nil {
		items := []*widget.FormItem{
			widget.NewFormItem("AccessKey", accessEntry),
			widget.NewFormItem("", widget.NewLabel("Paste the access key given by qiniu from the browser")),
			widget.NewFormItem("SecretKey", secretEntry),
			widget.NewFormItem("", widget.NewLabel("Paste below the secret key given by qiniu from the browser")),
			widget.NewFormItem("Bucket", bucketEntry),
			widget.NewFormItem("", widget.NewLabel("Paste below the bucket given by qiniu from the browser")),
		}
		gs.qNiuDialog = dialog.NewForm("七牛云 ", "上传", "取消", items,
			func(ok bool) {
				if ok {
					// 该函数，点击确认或者取消之后会调用
					if len(accessEntry.Text) == 0 {
						gs.status.SetText(fmt.Sprintf("check access enter this may null"))
						gs.qDrive.AccessKey = qiNiuAccessConfig
					} else {
						gs.App.Preferences().SetString(QiNiuAccessKey, accessEntry.Text)

						fmt.Println(gs.App.Preferences().String(QiNiuAccessKey))
						gs.qDrive.AccessKey = accessEntry.Text
					}

					if len(secretEntry.Text) == 0 {
						gs.status.SetText(fmt.Sprintf("check secret enter this may null"))
						gs.qDrive.SecretKey = qiNiuSecretConfig
					} else {
						gs.App.Preferences().SetString(QiNiuSecretKey, secretEntry.Text)
						gs.qDrive.SecretKey = secretEntry.Text
					}

					if len(bucketEntry.Text) == 0 {
						gs.status.SetText(fmt.Sprintf("check bucket enter this may null"))
						gs.qDrive.Bucket = qiNiuBucketConfig
					} else {
						gs.App.Preferences().SetString(QiNiuBucket, bucketEntry.Text)
						gs.qDrive.Bucket = bucketEntry.Text
					}
					// 开始传输操作
					fileName := gs.FireShotNameByTime()
					gs.qDriveNumShared++
					// 每次图片的名称要递增
					fileName = fmt.Sprintf("%s_%d.png", fileName, gs.qDriveNumShared)
					err := gs.qDrive.QiNiuShareImage(fileName, gs.Screenshot)
					if err != nil {
						gs.status.SetText(err.Error())
					} else {
						gs.status.SetText("图片上传成功 ...")
					}

				}
			}, gs.Win)
	}

	gs.qNiuDialog.Resize(fyne.NewSize(500, 300))
	gs.qNiuDialog.Show()
}

func (gs *FireShotGO) ShareWithGoogleDrive() {
	glog.V(2).Infof("FireShotGO.ShareWithGoogleDrive")
	ctx := context.Background()

	gs.status.SetText("开始连接谷歌云盘 ...")
	fileName := gs.DefaultName()
	gs.gDriveNumShared++
	if gs.gDriveNumShared > 1 {
		// In case the screenshot is shared multiple times (after different editions), we want
		// a different name for each.
		fileName = fmt.Sprintf("%s_%d", fileName, gs.gDriveNumShared)
	}

	go func() {
		if gs.gDrive == nil {
			// 创建云存储管理器.
			token := gs.App.Preferences().String(GoogleDriveTokenPreference)
			var err error
			gs.gDrive, err = cloud.New(ctx, GoogleDrivePath, token,
				func(token string) { gs.App.Preferences().SetString(GoogleDriveTokenPreference, token) },
				gs.askForGoogleDriveAuthorization)
			if err != nil {
				glog.Errorf("Failed to connect to Google Drive: %s", err)
				gs.status.SetText(fmt.Sprintf("GoogleDrive failed: %v", err))
				return
			}
		}

		// Sharing the image must happen in a separate goroutine because the UI must
		// remain interactive, also in order to capture the authorization input
		// from the user.
		url, err := gs.gDrive.ShareImage(ctx, fileName, gs.Screenshot)
		if err != nil {
			glog.Errorf("Failed to share image in Google Drive: %s", err)
			gs.status.SetText(fmt.Sprintf("GoogleDrive failed: %v", err))
			return
		}
		glog.Infof("GoogleDrive's shared URL:\t%s", url)
		err = clipboard.CopyText(url)
		if err == nil {
			gs.status.SetText("Image shared in GoogleDrive, URL copied to clipboard.")
		} else {
			gs.status.SetText("Image shared in GoogleDrive, but failed to copy to clipboard, see URL and error in the logs.")
			glog.Errorf("Failed to copy URL to clipboard: %v", err)
		}
	}()
}

func (gs *FireShotGO) askForGoogleDriveAuthorization() string {
	replyChan := make(chan string, 1)

	// Create dialog to get the authorization from the user.
	textEntry := widget.NewEntry()
	textEntry.Resize(fyne.NewSize(400, 40))
	items := []*widget.FormItem{
		widget.NewFormItem("Authorization", textEntry),
		widget.NewFormItem("", widget.NewLabel("Paste below the authorization given by GoogleDrive from the browser")),
	}
	form := dialog.NewForm("Google Drive Authorization", "Ok", "Cancel", items,
		func(confirm bool) {
			if confirm {
				replyChan <- textEntry.Text
			} else {
				replyChan <- ""
			}
		}, gs.Win)
	form.Resize(fyne.NewSize(500, 300))
	form.Show()
	gs.Win.Canvas().Focus(textEntry)

	return <-replyChan
}

// RegisterShortcuts adds all the shortcuts and keys FireShotGO
// listens to.
// When updating here, please update also the `fs.ShowShortcutsPage()`
// method to reflect the changes.
func (gs *FireShotGO) RegisterShortcuts() {
	gs.Win.Canvas().AddShortcut(
		&fyne.ShortcutCopy{},
		func(_ fyne.Shortcut) { gs.CopyImageToClipboard() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyJ, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(CropTopLeft) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(CropBottomRight) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawCircle) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyT, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawText) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: desktop.AltModifier},
		func(_ fyne.Shortcut) { gs.viewPort.SetOp(DrawArrow) })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.UndoLastFilter() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.SaveImage() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyG, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.ShareWithGoogleDrive() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: desktop.ControlModifier},
		func(_ fyne.Shortcut) { gs.ShowShortcutsPage() })
	gs.Win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: desktop.ControlModifier | desktop.ShiftModifier},
		func(_ fyne.Shortcut) { gs.ShowShortcutsPage() })

	gs.Win.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			if gs.viewPort.currentOperation != NoOp {
				gs.viewPort.SetOp(NoOp)
				gs.status.SetText("Operation cancelled.")
			}
			if gs.shortcutsDialog != nil {
				gs.shortcutsDialog.Hide()
			}
		} else {
			glog.V(2).Infof("KeyTyped: %+v", ev)
		}
	})
}

func (gs *FireShotGO) ShowShortcutsPage() {
	if gs.shortcutsDialog == nil {
		titleFn := func(title string) (l *widget.Label) {
			l = widget.NewLabel(title)
			l.TextStyle.Bold = true
			return l
		}
		descFn := func(desc string) (l *widget.Label) {
			l = widget.NewLabel(desc)
			l.Alignment = fyne.TextAlignCenter
			return l
		}
		shortcutFn := func(shortcut string) (l *widget.Label) {
			l = widget.NewLabel(shortcut)
			l.TextStyle.Italic = true
			return l
		}
		gs.shortcutsDialog = dialog.NewCustom("FireShotGO Shortcuts", "Ok",
			container.NewVScroll(container.NewVBox(
				titleFn("Image Manipulation"),
				container.NewGridWithColumns(2,
					descFn("Crop Top-Left"), shortcutFn("Alt+J"),
					descFn("Crop Bottom-Right"), shortcutFn("Alt+K"),
					descFn("Draw Circle"), shortcutFn("Alt+C"),
					descFn("Draw Arrow"), shortcutFn("Alt+A"),
					descFn("Draw Text"), shortcutFn("Alt+T"),
					descFn("Cancel Operation"), shortcutFn("Esc"),
					descFn("Undo Last Drawing"), shortcutFn("Control+Z"),
				),
				titleFn("Sharing Image"),
				container.NewGridWithColumns(2,
					descFn("Copy Image To Clipboard"), shortcutFn("Control+C"),
					descFn("Save Image"), shortcutFn("Control+S"),
					descFn("Google Drive & Copy URL"), shortcutFn("Control+G"),
				),
				titleFn("Other"),
				container.NewGridWithColumns(2,
					descFn("Shortcut page"), shortcutFn("Control+?"),
					descFn("Quit"), shortcutFn("Control+Q"),
				),
			)), gs.Win)
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	gs.shortcutsDialog.Resize(size)
	gs.shortcutsDialog.Show()
}

func (gs *FireShotGO) ConnectUsPage() {

	// TODO: 支持展示多个网格形状图片
	if gs.connectUsDialog == nil {
		weChatImage := canvas.NewImageFromResource(resources.WeChat)
		weChatContainer := container.NewScroll(weChatImage)
		weChatContainer.Resize(fyne.NewSize(420, 420))

		gs.connectUsDialog = dialog.NewCustom("FireShotGO", "确认",
			weChatContainer,
			gs.Win)

	}

	size := gs.Win.Canvas().Size()
	size.Width *= 0.80
	size.Height *= 0.80
	gs.connectUsDialog.Resize(size)
	gs.connectUsDialog.Show()
}

const DelayTimePreference = "DelayTime"
const SelectScreenIndex = "SelectScreen"

func (gs *FireShotGO) DelayedScreenshotForm() {
	if gs.delayedScreenshotDialog == nil {
		// 这里增加一个屏幕选择的窗口
		selectEntry := widget.NewEntry()
		selectEntry.Validator = validation.NewRegexp(`[1,2]`, "1 or 2 screen")
		se := gs.App.Preferences().Int(SelectScreenIndex)
		if se == 0 {
			se = 1
		}
		// 设置预写字段
		selectEntry.SetText(strconv.FormatInt(int64(se), 10))
		// 设置占位符，虽然这里自己只有两个屏幕但是为了避免有很多屏幕的情况，还是选择使用10进制
		selectEntry.SetPlaceHolder(strconv.FormatInt(int64(se), 10))

		// ----------------------------
		// 新弹出一个输入窗口
		delayEntry := widget.NewEntry()
		// 这里为输入窗口指定正则表达式式函数，一旦Validator为非空，窗口输入的所有内容将经过Validator指向的函数检测
		delayEntry.Validator = validation.NewRegexp(`\d`, "Must contain a number")
		// 使用App config全局配置参数获取参数，应用关闭也会有记录(如果设定过的话)
		v := gs.App.Preferences().Int(DelayTimePreference)
		if v == 0 {
			v = 5
		}
		// 填写预填写的数值，如果用户没有填写就替用户填写
		delayEntry.SetText(strconv.FormatInt(int64(v), 10))
		// 占位符，如果用户删除所有的内容，在Entry地方填写该数值
		delayEntry.SetPlaceHolder(strconv.FormatInt(int64(v), 10))
		// 创建新表单，点击文件-->延时截屏，弹出来该表单
		// 表单也改用中文，不再使用官方默认的英文
		gs.delayedScreenshotDialog = dialog.NewForm(
			"延时截屏",
			"确认", "取消",
			[]*widget.FormItem{
				widget.NewFormItem("输入屏幕序号 ",
					selectEntry),
				widget.NewFormItem("截屏延时 (s)",
					delayEntry),
			},
			func(ok bool) {
				if ok {
					// 获取并处理屏幕选择信息
					sn, err := strconv.ParseInt(selectEntry.Text, 10, 64)
					if err != nil {
						// 如果出错状态栏显示错误，状态栏目前挡放到了左下角，后期会调整到右下角
						gs.status.SetText(fmt.Sprintf("Can't parse screen no in sm from %q: %s",
							selectEntry.Text, err))
						glog.Errorf("Can't parse screen no in sm from %q: %s",
							selectEntry.Text, err)
						return
					}
					gs.App.Preferences().SetInt(SelectScreenIndex, int(sn))
					// 记录界面要输入的截屏序号，因为屏幕序号是从0开始的，因此输入的截屏序号只能是从[0-1]
					// 为了保持和电脑上计算屏幕的序号相同，这里代码中将序号调整
					gs.displayIndex = int(sn) - 1
					// 获取并处理延时信息 delayEntry.Text 是窗口输入的文本
					secs, err := strconv.ParseInt(delayEntry.Text, 10, 64)
					if err != nil {
						// FIXME : 调整状态信息
						// 如果出错状态栏显示错误，状态栏目前挡放到了左下角，后期会调整到右下角
						gs.status.SetText(fmt.Sprintf("Can't parse seconds in delay from %q: %s",
							delayEntry.Text, err))
						glog.Errorf("Can't parse seconds in delay from %q: %s",
							delayEntry.Text, err)
						return
					}
					// 填写的秒数，会通过App config，下次软件启动也能记录， 详情见我给出的教程：
					// @fyne_club https://gitee.com/andrewgithub/fyne-club/tree/master/bundle_data
					gs.App.Preferences().SetInt(DelayTimePreference, int(secs))

					// 开始延时截屏
					gs.DelayedScreenshot(int(secs))
				}
			}, gs.Win)
	}
	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	gs.delayedScreenshotDialog.Resize(size)
	gs.delayedScreenshotDialog.Show()
}

func (gs *FireShotGO) DelayedScreenshot(seconds int) {
	glog.V(2).Infof("DelayedScreenshot(%d secs)", seconds)
	go func() {
		for seconds > 0 {
			gs.status.SetText(fmt.Sprintf("Screenshot in %d seconds ...", seconds))
			time.Sleep(time.Second)
			seconds--
		}
		err := gs.MakeScreenshot()
		if err == nil {
			gs.status.SetText("New screenshot!")
		} else {
			glog.Errorf("Failed to create new screenshot: %v", err)
			gs.status.SetText(fmt.Sprintf("Failed to create new screenshot: %v", err))
		}
		gs.miniMap.updateViewPortRect()
		gs.viewPort.Refresh()
		gs.miniMap.Refresh()
	}()
}

func (gs *FireShotGO) GetStatusHandle() (status *widget.Label) {
	return gs.status
}
