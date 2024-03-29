package screenshot

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"gitee.com/andrewgithub/FireShotGo/filters"
	"gitee.com/andrewgithub/FireShotGo/resources"
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"
	"strconv"
)

// ViewPort is our view port for the image being edited. It's a specialized widget
// that will display the image according to zoom / window select.
//
// It is both a CanvasObject and a WidgetRenderer -- the abstractions in Fyne are
// not clear, but when those were implemented it worked (mostly copy&paste code).
//
// Loosely based on github.com/fyne-io/pixeledit
type ViewPort struct {
	widget.BaseWidget

	// fs 应用全局变量指针.
	fs *FireShotGO

	// Geometry of what is being displayed:
	// Log2Zoom is the log2 of the zoom multiplier, it's what we show to the user. It
	// is set by the "zoomEntry" field in the UI
	Log2Zoom float64

	// Thickness 存储绘制直线等元素的宽度元素.
	Thickness float64

	// DrawingColor 绘制图形的颜色信息. BackgroundColor text文本的背景颜色信息
	DrawingColor, BackgroundColor color.Color

	// FontSize 字体的大小
	FontSize float64

	// 虚线间隔配置
	DottedLineSpacing float64

	// Are of the screenshot that is visible in the current window: these are the start (viewX, viewY)
	// and sizes in fs.screenshot pixels -- each may be zoomed in/out when displaying.
	viewX, viewY, viewW, viewH int

	// Fyne objects.
	minSize fyne.Size
	// 样图，示例图
	raster *canvas.Raster

	// 鼠标后面跟随的图标
	cursor                                   *canvas.Image
	cursorCropTopLeft, cursorCropBottomRight *canvas.Image

	// 绘图工具区域
	cursorDrawCircle    *canvas.Image
	cursorDrawArrow     *canvas.Image
	cursorDrawText      *canvas.Image
	cursorDrawLine      *canvas.Image
	cursorDrawDotLine   *canvas.Image
	cursorShieldBlock   *canvas.Image
	cursorDrawRectangle *canvas.Image
	cursorDrawPen       *canvas.Image

	// 鼠标是否在视图窗口上
	mouseIn bool
	// 鼠标移动位置记录
	mouseMoveEvents chan fyne.Position

	// Cache image for current dimensions/zoom/translation.
	cache *image.RGBA

	// Dynamic dragging
	dragEvents chan *fyne.DragEvent
	// 记录首次开始拖拽时的位置
	dragStart                      fyne.Position
	dragStartViewX, dragStartViewY int
	dragSkipTap                    bool // Set at DragEnd(), because the end of the drag also triggers a tap.

	// 鼠标点击之后触发事件调用的 操作
	currentOperation    OperationType
	currentCircle       *filters.Circle       // 开始绘制圆, 只有当先设置 currentOperation==DrawCircle 之后才能使用
	currentArrow        *filters.Arrow        // 开始绘制箭头, 只有设置 currentOperation==DrawArrow 之后才能使用
	currentStraightLine *filters.StraightLine // 开始绘制直线 只有设置了 currentOperation==DrawStraightLine 之后才能使用
	currentDottedLine   *filters.DottedLine   // 开始绘制虚线
	currentShieldBlock  *filters.ShieldBlock  // 开始绘制矩形遮挡块
	currentRectangle    *filters.Rectangle    // 开始绘制矩形
	currentPen          *filters.Pen          // 开始使用画笔进行绘制

	fyne.ShortcutHandler
}

// OperationType 操作类型
type OperationType int

const (
	NoOp OperationType = iota
	// CropTopLeft 裁剪点 - 左上角
	CropTopLeft
	// CropBottomRight 裁剪点 - 右下角
	CropBottomRight
	// DrawCircle 绘制圆 包括椭圆和圆
	DrawCircle
	// DrawArrow 绘制剪头
	DrawArrow
	// DrawText 绘制文本
	DrawText
	// DrawStraightLine 绘制直线
	DrawStraightLine
	// DrawDottedLine 绘制虚线
	DrawDottedLine
	// DrawShieldBlock 绘制遮挡块
	DrawShieldBlock
	// DrawRectangle 绘制矩形
	DrawRectangle
	// DrawPen 使用画笔进行绘制
	DrawPen
)

// Ensure ViewPort implements the following interfaces.
var (
	vpPlaceholder = &ViewPort{}
	_             = fyne.CanvasObject(vpPlaceholder)
	_             = fyne.Draggable(vpPlaceholder)
	_             = fyne.Tappable(vpPlaceholder)
	_             = desktop.Hoverable(vpPlaceholder)
)

// NewViewPort 视窗，放置需要编辑的视图
func NewViewPort(gs *FireShotGO) (vp *ViewPort) {
	prefOrFloat := func(pref string, defaultValue float64) (value float64) {
		value = gs.App.Preferences().Float(pref)
		if value == 0 {
			value = defaultValue
		}
		return
	}

	vp = &ViewPort{
		fs: gs,
		// 绘图工具初始化区域,这里生成的是跟随绘图鼠标的缩略图标
		cursorCropTopLeft:     canvas.NewImageFromResource(resources.CropTopLeft),
		cursorCropBottomRight: canvas.NewImageFromResource(resources.CropBottomRight),
		cursorDrawCircle:      canvas.NewImageFromResource(resources.DrawCircle),
		cursorDrawArrow:       canvas.NewImageFromResource(resources.DrawArrow),
		cursorDrawText:        canvas.NewImageFromResource(resources.DrawText),
		cursorDrawLine:        canvas.NewImageFromResource(resources.DrawLine),
		cursorDrawDotLine:     canvas.NewImageFromResource(resources.DrawDottedLine),
		cursorShieldBlock:     canvas.NewImageFromResource(resources.DrawShieldBlock),
		cursorDrawRectangle:   canvas.NewImageFromResource(resources.DrawRectangle),
		cursorDrawPen:         canvas.NewImageFromResource(resources.DrawPen),
		// 记录鼠标位置信息
		mouseMoveEvents: make(chan fyne.Position, 1000),

		// 如果系统变量没有设置就使用默认值，字体大小
		FontSize: prefOrFloat(FontSizePreference, 16*float64(gs.Win.Canvas().Scale())),
		// 线条宽度，用于绘制线条的时候使用
		Thickness: prefOrFloat(ThicknessPreference, 3.0),
		// 绘制的颜色
		DrawingColor:    gs.GetColorPreference(DrawingColorPreference, Red),
		BackgroundColor: gs.GetColorPreference(BackgroundColorPreference, Transparent),
	}
	go vp.consumeMouseMoveEvents()
	vp.raster = canvas.NewRaster(vp.draw)
	return
}

const (
	BackgroundColorPreference = "BackgroundColor"
	DrawingColorPreference    = "DrawingColor"
	FontSizePreference        = "FontSize"
	ThicknessPreference       = "Thickness"
)

func (vp *ViewPort) Resize(size fyne.Size) {
	glog.V(2).Infof("Resize(size={w=%g, h=%g})", size.Width, size.Height)
	vp.BaseWidget.Resize(size)
	vp.raster.Resize(size)
}

func (vp *ViewPort) SetMinSize(size fyne.Size) {
	vp.minSize = size
}

func (vp *ViewPort) MinSize() fyne.Size {
	return vp.minSize
}

func (vp *ViewPort) CreateRenderer() fyne.WidgetRenderer {
	glog.V(2).Info("CreateRenderer()")
	return vp
}

func (vp *ViewPort) Destroy() {}

func (vp *ViewPort) Layout(size fyne.Size) {
	glog.V(2).Infof("Layout: size=(w=%g, h=%g)", size.Width, size.Height)
	// Resize to given size
	vp.raster.Resize(size)
}

func (vp *ViewPort) Refresh() {
	glog.V(2).Info("Refresh()")
	vp.renderCache()
	canvas.Refresh(vp)
}

func (vp *ViewPort) Objects() []fyne.CanvasObject {
	glog.V(3).Info("Objects()")
	if vp.cursor == nil || !vp.mouseIn {
		return []fyne.CanvasObject{vp.raster}
	}
	// 鼠标在视图上，并且鼠标后面跟随的图标已经初始化完成，预览图就返回当前的预览图和图标的叠加
	return []fyne.CanvasObject{vp.raster, vp.cursor}
}

// PixelSize returns the size in pixels of the this CanvasObject, based on the last request to redraw.
func (vp *ViewPort) PixelSize() (x, y int) {
	if vp.cache == nil {
		return 0, 0
	}
	return wh(vp.cache)
}

// PosToPixel converts from the undocumented Fyne screen float dimension to actual number of pixels
// position in the image.
func (vp *ViewPort) PosToPixel(pos fyne.Position) (x, y int) {
	fyneSize := vp.Size()
	pixelW, pixelH := vp.PixelSize()
	x = int((pos.X/fyneSize.Width)*float32(pixelW) + 0.5)
	y = int((pos.Y/fyneSize.Height)*float32(pixelH) + 0.5)
	return
}

func (vp *ViewPort) Scrolled(ev *fyne.ScrollEvent) {
	glog.V(2).Infof("Scrolled(dx=%f, dy=%f, position=%+v)", ev.Scrolled.DX, ev.Scrolled.DY, ev.Position)
	size := vp.Size()
	glog.V(2).Infof("- Size=%+v", vp.Size())
	w, h := wh(vp.cache)
	glog.V(2).Infof("- PxSize=(%d, %d)", w, h)
	pixelX, pixelY := vp.PosToPixel(ev.Position)
	glog.V(2).Infof("- Pixel position: (%d, %d)", pixelX, pixelY)

	// We want to scroll, but preserve the pixel from the screenshot being viewed at the mouse position.
	ratioX := ev.Position.X / size.Width
	ratioY := ev.Position.Y / size.Height
	screenshotX := int(ratioX*float32(vp.viewW) + float32(vp.viewX) + 0.5)
	screenshotY := int(ratioY*float32(vp.viewH) + float32(vp.viewY) + 0.5)
	glog.V(2).Infof("- Screenshot position: (%d, %d)", screenshotX, screenshotY)

	// Update zoom.
	vp.Log2Zoom += float64(ev.Scrolled.DY) / 50.0
	vp.fs.zoomEntry.SetText(fmt.Sprintf("%.3g", vp.Log2Zoom))

	// Update geometry.
	vp.updateViewSize()
	vp.viewX = screenshotX - int(ratioX*float32(vp.viewW)+0.5)
	vp.viewY = screenshotY - int(ratioY*float32(vp.viewH)+0.5)
	vp.Refresh()
	if vp.fs.miniMap != nil {
		vp.fs.miniMap.updateViewPortRect()
	}
}

func (vp *ViewPort) updateViewSize() {
	zoom := vp.zoom()
	pixelW, pixelH := vp.PixelSize()
	vp.viewW = int(float64(pixelW)*zoom + 0.5)
	vp.viewH = int(float64(pixelH)*zoom + 0.5)
}

// Draw implements canvas.Raster Generator: it generates the image that will be drawn.
// The image should already be rendered in vp.cache, but this handles exception cases.
func (vp *ViewPort) draw(w, h int) image.Image {
	glog.V(2).Infof("draw(w=%d, h=%d)", w, h)
	currentW, currentH := vp.PixelSize()
	if currentW == w && currentH == h {
		// Cache is good, reuse it.
		glog.V(2).Infof("- reuse")
		return vp.cache
	}

	// Regenerate cache.
	vp.cache = image.NewRGBA(image.Rect(0, 0, w, h))
	vp.updateViewSize()
	if vp.fs.miniMap != nil {
		vp.fs.miniMap.updateViewPortRect()
	}
	vp.renderCache()
	return vp.cache
}

// wh extracts the width and height of an image.
func wh(img image.Image) (int, int) {
	if img == nil {
		return 0, 0
	}
	rect := img.Bounds()
	return rect.Dx(), rect.Dy()
}

func (vp *ViewPort) zoom() float64 {
	return math.Exp2(-vp.Log2Zoom)
}

func (vp *ViewPort) renderCache() {
	const bytesPerPixel = 4 // RGBA.
	w, h := wh(vp.cache)
	img := vp.fs.Screenshot
	imgW, imgH := wh(img)
	zoom := vp.zoom()

	var c color.RGBA
	glog.V(2).Infof("renderCache(): cache=(w=%d, h=%d, bytes=%d), zoom=%g, viewX=%d, viewY=%d, viewW=%d, viewH=%d",
		w, h, len(vp.cache.Pix), zoom, vp.viewX, vp.viewY, vp.viewW, vp.viewH)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := (y*w + x) * bytesPerPixel
			imgX := int(math.Round(float64(x)*zoom)) + vp.viewX
			imgY := int(math.Round(float64(y)*zoom)) + vp.viewY
			if imgX < 0 || imgX >= imgW || imgY < 0 || imgY >= imgH {
				// Background image.
				c = bgPattern(x, y)
			} else {
				c = img.RGBAAt(imgX, imgY)
			}
			vp.cache.Pix[pos] = c.R
			vp.cache.Pix[pos+1] = c.G
			vp.cache.Pix[pos+2] = c.B
			vp.cache.Pix[pos+3] = c.A
		}
	}
}

var (
	bgDark, bgLight = color.RGBA{R: 58, G: 58, B: 58, A: 0xFF}, color.RGBA{R: 84, G: 84, B: 84, A: 0xFF}
)

func bgPattern(x, y int) color.RGBA {
	const boxSize = 25
	if (x/boxSize)%2 == (y/boxSize)%2 {
		return bgDark
	}
	return bgLight
}

// ===============================================================
// Implementation of dragging view window on ViewPort
// ===============================================================

// Dragged implements fyne.Draggable
// 拖动时候将对应图像实现放入到fileter中
func (vp *ViewPort) Dragged(ev *fyne.DragEvent) {
	if vp.dragEvents == nil {
		glog.V(2).Infof("Dragged(): start new drag for Op=%d", vp.currentOperation)
		// Create a channel to send dragEvents and start goroutine to consume them sequentially.
		vp.dragEvents = make(chan *fyne.DragEvent, dragEventsQueue)
		vp.dragStart = ev.Position
		vp.dragStartViewX = vp.viewX
		vp.dragStartViewY = vp.viewY
		go vp.consumeDragEvents()

		startX, startY := vp.screenshotPos(vp.dragStart)
		startX += vp.fs.CropRect.Min.X
		startY += vp.fs.CropRect.Min.Y

		switch vp.currentOperation {
		case NoOp, CropTopLeft, CropBottomRight, DrawText:
			// Drag the image around, nothing to do to start.
		case DrawCircle:
			glog.V(2).Infof("Tapped(): draw a circle starting at (%d, %d)", startX, startY)
			vp.currentCircle = filters.NewCircle(image.Rectangle{
				Min: image.Point{X: startX, Y: startY},
				Max: image.Point{X: startX + 5, Y: startY + 5},
			}, vp.DrawingColor, vp.Thickness)
			vp.fs.Filters = append(vp.fs.Filters, vp.currentCircle)
			vp.fs.ApplyFilters(false)
		case DrawArrow:
			glog.V(2).Infof("Tapped(): draw an arrow starting at (%d, %d)", startX, startY)
			vp.currentArrow = filters.NewArrow(
				image.Point{X: startX, Y: startY},
				image.Point{X: startX + 1, Y: startY + 1},
				vp.DrawingColor, vp.Thickness)
			vp.fs.Filters = append(vp.fs.Filters, vp.currentArrow)
			vp.fs.ApplyFilters(false)

		case DrawStraightLine:
			glog.V(2).Infof("Tapped(): draw an Line starting at (%d, %d)", startX, startY)
			vp.currentStraightLine = filters.NewStraightLine(
				image.Point{X: startX, Y: startY},
				image.Point{X: startX + 1, Y: startY + 1},
				vp.DrawingColor, vp.Thickness)

			vp.fs.Filters = append(vp.fs.Filters, vp.currentStraightLine)
			vp.fs.ApplyFilters(false)
		case DrawDottedLine:
			glog.V(2).Infof("Tapped(): draw an Line starting at (%d, %d)", startX, startY)
			vp.currentDottedLine = filters.NewDottedLine(
				image.Point{X: startX, Y: startY},
				image.Point{X: startX + 1, Y: startY + 1},
				vp.DrawingColor, vp.Thickness, vp.DottedLineSpacing)

			vp.fs.Filters = append(vp.fs.Filters, vp.currentDottedLine)
			vp.fs.ApplyFilters(false)
		case DrawShieldBlock:
			glog.V(2).Infof("Tapped(): draw a shield block starting at (%d, %d)", startX, startY)
			vp.currentShieldBlock = filters.NewShieldBlock(image.Rectangle{
				Min: image.Point{X: startX, Y: startY},
				Max: image.Point{X: startX + 5, Y: startY + 5},
			}, vp.DrawingColor)
			vp.fs.Filters = append(vp.fs.Filters, vp.currentShieldBlock)
			vp.fs.ApplyFilters(false)
		case DrawRectangle:
			glog.V(2).Infof("Tapped(): draw a rectangle starting at (%d, %d)", startX, startY)
			vp.currentRectangle = filters.NewRectangle(image.Rectangle{
				Min: image.Point{X: startX, Y: startY},
				Max: image.Point{X: startX + 5, Y: startY + 5},
			}, vp.DrawingColor,
				vp.Thickness)
			vp.fs.Filters = append(vp.fs.Filters, vp.currentRectangle)
			vp.fs.ApplyFilters(false)
		case DrawPen:
			glog.V(2).Infof("Tapped(): draw a line at (%d, %d)", startX, startY)
			vp.currentPen = filters.NewPen(image.Point{startX, startY}, vp.DrawingColor,
				vp.Thickness)
			vp.fs.Filters = append(vp.fs.Filters, vp.currentPen)
			vp.fs.ApplyFilters(false)
		}

		return // No need to process first event.
	}
	vp.dragEvents <- ev
	vp.mouseMoveEvents <- ev.Position // Also emits a mouse move event.
}

func (vp *ViewPort) consumeDragEvents() {
	var prevDragPos fyne.Position
	for done := false; !done; {
		// Wait for something to happen.
		ev := <-vp.dragEvents
		if ev == nil {
			// All done.
			break
		}

		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	drainDragEvents:
		for {
			select {
			case newEvent := <-vp.dragEvents:
				if newEvent == nil {
					// Channel closed, but we still need to process last event.
					done = true
					break drainDragEvents // Emptied the channel.
				} else {
					// New event arrived.
					consumed++
					ev = newEvent
				}
			default:
				break drainDragEvents // Emptied the channel.
			}
		}
		if ev != nil {
			if ev.Position != prevDragPos {
				prevDragPos = ev.Position
				glog.V(2).Infof("consumeDragEvents(pos=%+v, consumed=%d)", ev.Position, consumed)
				vp.doDragThrottled(ev)
			}
		}
	}
	vp.dragStart = fyne.Position{}
	glog.V(2).Info("consumeDragEvents(): done")
}

// doDragThrottled is called sequentially, dropping drag events in between each call. So
// each time it is called with the latest DragEvent, dropping those that happened in between
// the previous call.
// 随着鼠标拖动实时更新end point
func (vp *ViewPort) doDragThrottled(ev *fyne.DragEvent) {
	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// 当NoOp时，裁剪，或者文本时，如果单击鼠标进行拖动就拖动图片
		vp.dragViewDelta(ev.Position.Subtract(vp.dragStart))
	case DrawCircle:
		vp.dragCircle(ev.Position)
	case DrawArrow:
		vp.dragArrow(ev.Position)
	case DrawStraightLine:
		vp.dragLine(ev.Position)
	case DrawDottedLine:
		vp.dragDottedLine(ev.Position)
	case DrawShieldBlock:
		vp.dragShieldBlock(ev.Position)
	case DrawRectangle:
		vp.dragRectangle(ev.Position)
	case DrawPen:
		vp.DragPen(ev.Position)
	}
}

func (vp *ViewPort) dragViewDelta(delta fyne.Position) {
	size := vp.Size()

	ratioX := delta.X / size.Width
	ratioY := delta.Y / size.Height

	vp.viewX = vp.dragStartViewX - int(ratioX*float32(vp.viewW)+0.5)
	vp.viewY = vp.dragStartViewY - int(ratioY*float32(vp.viewH)+0.5)
	vp.Refresh()
	vp.fs.miniMap.updateViewPortRect()
}

func (vp *ViewPort) dragCircle(toPos fyne.Position) {
	if vp.currentCircle == nil {
		glog.Errorf("dragCircle(): dragCircle event, but none has been started yet!?")
	}
	startX, startY := vp.screenshotPos(vp.dragStart)
	startX += vp.fs.CropRect.Min.X
	startY += vp.fs.CropRect.Min.Y
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	vp.currentCircle.SetDim(image.Rectangle{
		Min: image.Point{X: startX, Y: startY},
		Max: image.Point{X: toX, Y: toY},
	}.Canon())
	glog.V(2).Infof("dragCircle(): draw a circle in %+v", vp.currentCircle)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

func (vp *ViewPort) dragArrow(toPos fyne.Position) {
	if vp.currentArrow == nil {
		glog.Errorf("dragArrow(): dragArrow event, but none has been started yet!?")
	}
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	vp.currentArrow.SetPoints(vp.currentArrow.From, image.Point{X: toX, Y: toY})
	glog.V(2).Infof("dragArrow(): draw an arrow in %+v", vp.currentArrow)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// dragLine 当前窗口的左上角位置
func (vp *ViewPort) dragLine(toPos fyne.Position) {
	if vp.currentStraightLine == nil {
		glog.Errorf("dragLine(): dragLine event, but none has been started yet!?")
	}
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	vp.currentStraightLine.SetPoints(vp.currentStraightLine.From, image.Point{X: toX, Y: toY})
	glog.V(2).Infof("dragStraightLine(): draw an line in %+v", vp.currentStraightLine)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// dragDottedLine 当前窗口的左上角位置
func (vp *ViewPort) dragDottedLine(toPos fyne.Position) {
	if vp.currentDottedLine == nil {
		glog.Errorf("dragLine(): dragDottedLine event, but none has been started yet!?")
	}
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	vp.currentDottedLine.SetPoints(vp.currentDottedLine.From, image.Point{X: toX, Y: toY})
	glog.V(2).Infof("dragDottedLine(): draw an dot line in %+v", vp.currentDottedLine)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

func (vp *ViewPort) DragPen(toPos fyne.Position) {
	if vp.currentPen == nil {
		glog.Errorf("dragPen(): dragPen event, but none has been started yet!?")
	}
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	// 设置进去的rect已经保证max > min
	vp.currentPen.SetPoints(image.Point{
		toX, toY,
	})
	glog.V(2).Infof("dragPen(): draw a point in %+v", vp.currentPen)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// dragShieldBlock 当前窗口的左上角位置
func (vp *ViewPort) dragShieldBlock(toPos fyne.Position) {
	if vp.currentShieldBlock == nil {
		glog.Errorf("dragShieldBlock(): dragShieldBlock event, but none has been started yet!?")
	}
	startX, startY := vp.screenshotPos(vp.dragStart)
	startX += vp.fs.CropRect.Min.X
	startY += vp.fs.CropRect.Min.Y
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	// 设置进去的rect已经保证max > min
	vp.currentShieldBlock.SetRect(image.Rectangle{
		Min: image.Point{X: startX, Y: startY},
		Max: image.Point{X: toX, Y: toY},
	}.Canon())
	glog.V(2).Infof("dragCircle(): draw a circle in %+v", vp.currentShieldBlock)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// dragShieldBlock 当前窗口的左上角位置
func (vp *ViewPort) dragRectangle(toPos fyne.Position) {
	if vp.currentRectangle == nil {
		glog.Errorf("dragRectangle(): dragRectangle event, but none has been started yet!?")
	}
	startX, startY := vp.screenshotPos(vp.dragStart)
	startX += vp.fs.CropRect.Min.X
	startY += vp.fs.CropRect.Min.Y
	toX, toY := vp.screenshotPos(toPos)
	toX += vp.fs.CropRect.Min.X
	toY += vp.fs.CropRect.Min.Y
	// 设置进去的rect已经保证max > min
	vp.currentRectangle.SetRect(image.Rectangle{
		Min: image.Point{X: startX, Y: startY},
		Max: image.Point{X: toX, Y: toY},
	}.Canon())
	glog.V(2).Infof("dragCircle(): draw a circle in %+v", vp.currentRectangle)
	vp.fs.ApplyFilters(false)
	vp.renderCache()
	vp.Refresh()
}

// DragEnd implements fyne.Draggable
func (vp *ViewPort) DragEnd() {
	glog.V(2).Infof("DragEnd(), dragEvents=%v", vp.dragEvents != nil)
	close(vp.dragEvents)

	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// Drag the image around, nothing to do to start.
	case DrawCircle, DrawArrow, DrawStraightLine, DrawDottedLine, DrawShieldBlock, DrawRectangle, DrawPen:
		vp.fs.ApplyFilters(true)
	}
	vp.dragEvents = nil
	vp.dragSkipTap = true

	switch vp.currentOperation {
	case NoOp, CropTopLeft, CropBottomRight, DrawText:
		// Nothing to do
	case DrawPen:
		vp.fs.status.SetText("Drawing done, use Control+Z to undo.")
		vp.SetOp(NoOp)

	case DrawCircle, DrawArrow, DrawStraightLine, DrawDottedLine, DrawShieldBlock, DrawRectangle:
		vp.currentCircle = nil
		vp.currentArrow = nil
		vp.currentStraightLine = nil
		vp.currentDottedLine = nil
		vp.currentShieldBlock = nil
		vp.currentRectangle = nil
		vp.fs.status.SetText("Drawing done, use Control+Z to undo.")
		vp.SetOp(NoOp)
	}
}

// ===============================================================
// Implementation of a cursor on ViewPort
// ===============================================================
//func (vp *ViewPort) Set

// MouseIn implements desktop.Hoverable.
func (vp *ViewPort) MouseIn(ev *desktop.MouseEvent) {
	vp.mouseIn = true
	if vp.cursor != nil {
		vp.cursor.Move(ev.Position)
	}
}

// MouseMoved implements desktop.Hoverable.
func (vp *ViewPort) MouseMoved(ev *desktop.MouseEvent) {
	if vp.cursor != nil {
		// Send event to channel, it will only be acted on in
		// vp.processMouseMoveEvent.
		vp.mouseMoveEvents <- ev.Position
	}
}

// MouseOut implements desktop.Hoverable.
func (vp *ViewPort) MouseOut() {
	vp.mouseIn = false
}

// processMouseMoveEvent is the function that actually acts on a
// mouse movement event.
func (vp *ViewPort) processMouseMoveEvent(pos fyne.Position) {
	if vp.cursor != nil {
		vp.cursor.Move(pos)
		vp.Refresh()
	}
}

// consumeMouseMoveEvents 等待鼠标事件并处理
func (vp *ViewPort) consumeMouseMoveEvents() {
	// vp.mouseMoveEvents 只有进程退出该GoRoutine才会退出
	for {
		// Wait for something to happen.
		ev, ok := <-vp.mouseMoveEvents
		if !ok {
			return
		}

		// Read all events in channel, until it blocks or is closed.
		consumed := 0
	mouseMoveEventsLoop:
		for {
			select {
			case newEvent, ok := <-vp.mouseMoveEvents:
				if !ok {
					return
				}

				// New event arrived.
				consumed++
				ev = newEvent
			default:
				break mouseMoveEventsLoop
			}
		}
		vp.processMouseMoveEvent(ev)
	}
}

// ===============================================================
// Implementation of operations on ViewPort, 鼠标跟随图标大小
// ===============================================================
var cursorSize = fyne.NewSize(32, 32)

// SetOp changes the current op on the edit window. It interrupts any dragging event going on.
// 在编辑窗口选择需要进行的操作，比如选择绘制直线
func (vp *ViewPort) SetOp(op OperationType) {
	if vp.dragEvents != nil {
		vp.DragEnd()
	}
	vp.currentOperation = op
	switch op {
	case NoOp:
		if vp.cursor != nil {
			vp.cursor = nil
			vp.Refresh()
		}

	case CropTopLeft:
		vp.cursor = vp.cursorCropTopLeft
		vp.cursor.Resize(cursorSize)

	case CropBottomRight:
		vp.cursor = vp.cursorCropBottomRight
		vp.cursor.Resize(cursorSize)

	case DrawCircle:
		vp.cursor = vp.cursorDrawCircle
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag to draw circle!")

	case DrawArrow:
		vp.cursor = vp.cursorDrawArrow
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw an arrow!")

	case DrawText:
		vp.cursor = vp.cursorDrawText
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click to define center location of text.")

	case DrawStraightLine:
		vp.cursor = vp.cursorDrawLine
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw an line!")

	case DrawDottedLine:
		vp.cursor = vp.cursorDrawDotLine
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw an dotted line!")

	case DrawShieldBlock:
		vp.cursor = vp.cursorShieldBlock
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw an shield block!")

	case DrawRectangle:
		vp.cursor = vp.cursorDrawRectangle
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw an rectangle!")
	case DrawPen:
		vp.cursor = vp.cursorDrawPen
		vp.cursor.Resize(cursorSize)
		vp.fs.status.SetText("Click and drag from start to end (point side) to draw some points!")
	}

}

// screenshotPos returns the screenshot position for the given
// position in the canvas.
func (vp *ViewPort) screenshotPos(pos fyne.Position) (x, y int) {
	size := vp.Size()
	ratioX := pos.X / size.Width
	ratioY := pos.Y / size.Height
	x = int(ratioX*float32(vp.viewW) + float32(vp.viewX) + 0.5)
	y = int(ratioY*float32(vp.viewH) + float32(vp.viewY) + 0.5)
	return
}

func (vp *ViewPort) Tapped(ev *fyne.PointEvent) {
	glog.V(2).Infof("Tapped(pos=%+v, op=%d), dragSkipTag=%v", ev.Position, vp.currentOperation, vp.dragSkipTap)
	if vp.dragSkipTap {
		// End of a drag, we discard this tap.
		vp.dragSkipTap = false
		return
	}
	screenshotX, screenshotY := vp.screenshotPos(ev.Position)
	screenshotPoint := image.Point{X: screenshotX, Y: screenshotY}
	absolutePoint := screenshotPoint.Add(vp.fs.CropRect.Min)

	switch vp.currentOperation {
	case NoOp:
		// Nothing ...
	case CropTopLeft:
		vp.cropTopLeft(screenshotX, screenshotY)
	case CropBottomRight:
		vp.cropBottomRight(screenshotX, screenshotY)
	case DrawCircle, DrawArrow, DrawStraightLine, DrawDottedLine, DrawShieldBlock, DrawRectangle, DrawPen:
		vp.fs.status.SetText("You must drag to draw something ...")
	case DrawText:
		vp.createTextFilter(absolutePoint)
	}

	// After a tap
	vp.SetOp(NoOp)
}

func (vp *ViewPort) createTextFilter(center image.Point) {
	var form dialog.Dialog
	textEntry := widget.NewMultiLineEntry()
	textEntry.Resize(fyne.NewSize(400, 80))
	fontSize := widget.NewEntry()
	fontSize.SetText(fmt.Sprintf("%g", vp.FontSize))
	fontSize.Validator = validation.NewRegexp(`\d`, "Must contain a number")
	bgColorRect := canvas.NewRectangle(vp.BackgroundColor)
	bgColorRect.SetMinSize(fyne.NewSize(200, 20))
	picker := dialog.NewColorPicker(
		"Pick a Color", "Select background color for text",
		func(c color.Color) {
			vp.BackgroundColor = c
			vp.fs.SetColorPreference(BackgroundColorPreference, c)
			bgColorRect.FillColor = vp.BackgroundColor
			bgColorRect.Refresh()
			form.Refresh()
		},
		vp.fs.Win)
	backgroundEntry := container.NewHBox(
		// Set color button.
		widget.NewButtonWithIcon("", resources.ColorWheel, func() { picker.Show() }),
		// No color button
		widget.NewButtonWithIcon("", resources.Reset, func() {
			vp.BackgroundColor = Transparent
			bgColorRect.FillColor = vp.BackgroundColor
			bgColorRect.Refresh()
		}),
		bgColorRect,
	)
	items := []*widget.FormItem{
		widget.NewFormItem("Text", textEntry),
		widget.NewFormItem("Font size", fontSize),
		widget.NewFormItem("Background", backgroundEntry),
	}
	form = dialog.NewForm("Insert text", "Ok", "Cancel", items,
		func(confirm bool) {
			if confirm {
				fSize, err := strconv.ParseFloat(fontSize.Text, 64)
				if err != nil {
					glog.Errorf("Error parsing the font size given: %q", fontSize.Text)
					vp.fs.status.SetText(fmt.Sprintf("Error parsing the font size given: %q", fontSize.Text))
					return
				}
				vp.FontSize = fSize
				vp.fs.App.Preferences().SetFloat(FontSizePreference, fSize)
				textFilter := filters.NewText(textEntry.Text, center, vp.DrawingColor, vp.BackgroundColor, fSize)
				vp.fs.Filters = append(vp.fs.Filters, textFilter)
				vp.fs.ApplyFilters(true)
				vp.fs.status.SetText("Text drawn, use Control+Z to undo.")
			}
		}, vp.fs.Win)
	form.Resize(fyne.NewSize(500, 300))
	form.Show()
	vp.fs.Win.Canvas().Focus(textEntry)
}

// cropTopLeft will crop the screenshot on this position.
func (vp *ViewPort) cropTopLeft(x, y int) {
	vp.fs.CropRect.Min = vp.fs.CropRect.Min.Add(image.Point{X: x, Y: y})
	vp.fs.ApplyFilters(true)
	vp.viewX, vp.viewY = 0, 0 // Move view to cropped corner.
	glog.V(2).Infof("cropTopLeft: new cropRect is %+v", vp.fs.CropRect)
	vp.postCrop()
}

// cropBottomRight will crop the screenshot on this position.
func (vp *ViewPort) cropBottomRight(x, y int) {
	vp.fs.CropRect.Max = vp.fs.CropRect.Max.Sub(
		image.Point{X: vp.fs.CropRect.Dx() - x, Y: vp.fs.CropRect.Dy() - y})
	vp.fs.ApplyFilters(true)
	vp.viewX, vp.viewY = x-vp.viewW, y-vp.viewH // Move view to cropped corner.
	vp.postCrop()
}

func (vp *ViewPort) cropReset() {
	vp.viewX += vp.fs.CropRect.Min.X
	vp.viewY += vp.fs.CropRect.Min.Y
	vp.fs.CropRect = vp.fs.OriginalScreenshot.Rect
	vp.fs.ApplyFilters(true)
	vp.postCrop()
	vp.fs.status.SetText(fmt.Sprintf("Reset to original screenshot of size %d x %d pixels.",
		vp.fs.CropRect.Dx(), vp.fs.CropRect.Dy()))
}

// postCrop refreshes elements after a change in crop.
func (vp *ViewPort) postCrop() {
	// Full image fits the view port in any of the dimensions, then we center the image.
	if vp.fs.CropRect.Dx() < vp.viewW {
		vp.viewX = -(vp.viewW - vp.fs.CropRect.Dx()) / 2
	}
	if vp.fs.CropRect.Dy() < vp.viewH {
		vp.viewY = -(vp.viewH - vp.fs.CropRect.Dy()) / 2
	}

	vp.updateViewSize()
	vp.renderCache()
	vp.Refresh()
	vp.fs.miniMap.updateViewPortRect()
	vp.fs.miniMap.Refresh()
	vp.fs.status.SetText(fmt.Sprintf("New crop: {%d, %d} - {%d, %d} of original screen, %d x %d pixels.",
		vp.fs.CropRect.Min.X, vp.fs.CropRect.Min.Y, vp.fs.CropRect.Max.X, vp.fs.CropRect.Max.Y,
		vp.fs.CropRect.Dx(), vp.fs.CropRect.Dy()))
}
