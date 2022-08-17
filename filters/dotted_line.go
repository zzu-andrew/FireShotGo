package filters

import (
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type DottedLine struct {
	// From, To 虚线开始到结束的位置记录:
	From, To image.Point

	// Color 指定虚线的颜色.
	Color color.Color

	// Thickness 指定虚线的宽度
	Thickness float64

	// Rectangle enclosing dotted line.
	rect image.Rectangle
	// 转换矩阵
	rebaseMatrix mgl64.Mat3
	// 按照矢量求出当前绘制的长度
	vectorLength float64
}

// NewDottedLine 创建一个新的虚线，接口中必须传入虚线的宽度、颜色以及起点.
func NewDottedLine(from, to image.Point, color color.Color, thickness float64) *DottedLine {
	c := &DottedLine{Color: color, Thickness: thickness}
	c.SetPoints(from, to)
	return c
}

// SetPoints 虚线打点
func (c *DottedLine) SetPoints(from, to image.Point) {
	if to.X == from.X && to.Y == from.Y {
		to.X += 1 // 保证虚线最少为一个像素大小，否则图像上显示不出来
	}
	c.From, c.To = from, to
	// rect 确保最小值小于最大值,保证由Min 指向Max 如果不是就反转
	c.rect = image.Rectangle{Min: from, Max: to}.Canon()
	// 保证线条的宽度，在做矩阵转换的时候能保证起点和结束点的宽度一致
	// 设置线条的宽度
	headExtraPixels := int(c.Thickness)
	c.rect.Min.X -= headExtraPixels
	c.rect.Min.Y -= headExtraPixels
	c.rect.Max.X += headExtraPixels
	c.rect.Max.Y += headExtraPixels

	// 求出从开始到结束的delta，随着鼠标的拖动会一直触发
	delta := c.To.Sub(c.From)
	//fmt.Println(delta)
	vector := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
	// 求出当前虚线的长度
	c.vectorLength = vector.Len()
	// 求出虚线的方向
	direction := vector.Mul(1.0 / c.vectorLength)
	// 角度
	angle := math.Atan2(direction.Y(), direction.X())
	glog.V(2).Infof("SetPoints(from=%v, to=%v): delta=%v, length=%.0f, angle=%5.1f",
		from, to, delta, c.vectorLength, mgl64.RadToDeg(angle))
	//fmt.Println(angle)
	c.rebaseMatrix = mgl64.HomogRotate2D(-angle)
	c.rebaseMatrix = c.rebaseMatrix.Mul3(
		mgl64.Translate2D(float64(-c.From.X), float64(-c.From.Y)))
}

// at is the function given to the filterImage object.
// under 是当前背景图片上的当前颜色
func (c *DottedLine) at(x, y int, under color.Color) color.Color {
	if x > c.rect.Max.X || x < c.rect.Min.X || y > c.rect.Max.Y || y < c.rect.Min.Y {
		return under
	}

	// Move to coordinates on the segment defined from c.From to c.To.
	homogPoint := mgl64.Vec3{float64(x), float64(y), 1.0} // Homogeneous coordinates.
	if glog.V(3) {
		if math.Abs(homogPoint.Y()-float64(c.To.Y)) < 2 || math.Abs(homogPoint.X()-float64(c.To.X)) < 2 {
			return Yellow
		}
		if math.Abs(homogPoint.Y()-float64(c.From.Y)) < 2 || math.Abs(homogPoint.X()-float64(c.From.X)) < 2 {
			return Yellow
		}
	}
	homogPoint = c.rebaseMatrix.Mul3x1(homogPoint)
	if glog.V(3) {
		if math.Abs(homogPoint.Y()) < 3 {
			return Green
		}
		if math.Abs(homogPoint.X()) < 1 {
			return Green
		}
		if math.Abs(homogPoint.X()-c.vectorLength) < 1 {
			return Green
		}
	}

	if homogPoint.X() < 0 {
		return under
	}

	if homogPoint.X() < c.vectorLength {
		if math.Abs(homogPoint.Y()) < c.Thickness/2 {
			if (int)(homogPoint.X()/5)%2 == 0 {
				return c.Color
			}
		}
	}

	return under
}

// Apply 接口ImageFilter的实现.
// 实现方式，若是需要绘制的图，就替换为当先选中的颜色，若是不是就返回背景颜色 under
func (c *DottedLine) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
