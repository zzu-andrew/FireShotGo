package resources

// This file embeds all the resources used by the program.

import (
	_ "embed"
	"fyne.io/fyne/v2"
)

//go:embed reset.png
var embedReset []byte
var Reset = fyne.NewStaticResource("reset", embedReset)

//go:embed crop_top_left.png
var embedCropTopLeft []byte
var CropTopLeft = fyne.NewStaticResource("", embedCropTopLeft)

//go:embed crop_bottom_right.png
var embedCropBottomRight []byte
var CropBottomRight = fyne.NewStaticResource("", embedCropBottomRight)

//go:embed draw_circle.png
var embedDrawCircle []byte
var DrawCircle = fyne.NewStaticResource("", embedDrawCircle)

//go:embed draw_arrow.png
var embedDrawArrow []byte
var DrawArrow = fyne.NewStaticResource("", embedDrawArrow)

//go:embed draw-line.png
var embedDrawLine []byte
var DrawLine = fyne.NewStaticResource("", embedDrawLine)

//go:embed draw-dot-line.png
var embedDrawDotted []byte
var DrawDottedLine = fyne.NewStaticResource("", embedDrawDotted)

//go:embed shield_block.png
var embedDrawShieldBlock []byte
var DrawShieldBlock = fyne.NewStaticResource("", embedDrawShieldBlock)

//go:embed pen.png
var embedDrawPen []byte
var DrawPen = fyne.NewStaticResource("", embedDrawPen)

//go:embed draw_rectangle.png
var embedDrawRectangle []byte
var DrawRectangle = fyne.NewStaticResource("", embedDrawRectangle)

//go:embed draw_text.png
var embedDrawText []byte
var DrawText = fyne.NewStaticResource("", embedDrawText)

//go:embed thickness.png
var embedThickness []byte
var Thickness = fyne.NewStaticResource("Thickness", embedThickness)

//go:embed colors.png
var embedColors []byte
var Colors = fyne.NewStaticResource("Colors", embedColors)

//go:embed colorwheel.png
var embedColorWheel []byte
var ColorWheel = fyne.NewStaticResource("ColorWheel", embedColorWheel)

//go:embed fire.png
var embedGoShotIconPng []byte
var GoShotIconPng = fyne.NewStaticResource("GoShotIconPng", embedGoShotIconPng)

//go:embed weixin.png
var weChatPic []byte
var WeChat = fyne.NewStaticResource("GoShotIconIco", weChatPic)
