package filters

import (
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Arrow struct {
	// From, To 箭头的起始点:
	// 箭头在To的位置
	From, To image.Point

	// Color 定义箭头的颜色
	Color color.Color

	// Thickness 箭头的宽度 实际像素点为 Thickness*arrowHeadWidthFactor
	Thickness float64

	// rect 箭头的矩形部分 (箭头 + 矩形线框组成一条箭头线)
	rect image.Rectangle
	// 现行[3x3]矩阵
	rebaseMatrix mgl64.Mat3
	// 箭头需要绘制的长度
	vectorLength float64
}

// arrowHeadLengthFactor 箭头长度预设
// arrowHeadWidthFactor 箭头宽度预设
const (
	arrowHeadLengthFactor = 10.0
	arrowHeadWidthFactor  = 6.0
)

// NewArrow 创建一个箭头，必须传入起始点和颜色，以及线条的宽度
func NewArrow(from, to image.Point, color color.Color, thickness float64) *Arrow {
	c := &Arrow{Color: color, Thickness: thickness}
	c.SetPoints(from, to)
	return c
}

func (c *Arrow) SetPoints(from, to image.Point) {
	// 不支持点，最少绘制一个长度才能画出箭头
	if to.X == from.X && to.Y == from.Y {
		to.X += 1 // So that arrow is always at least 1 in size.
	}
	// 将起始位置设置给箭头
	c.From, c.To = from, to
	// rect 中记录起始位置，位置信息时经过Canon Min.X < Max.X and Min.Y < Max.Y
	c.rect = image.Rectangle{Min: from, Max: to}.Canon()
	// 取出线宽，只要> 0.1就进位
	arrowHeadExtraPixels := int(arrowHeadWidthFactor*c.Thickness + 0.99)
	c.rect.Min.X -= arrowHeadExtraPixels
	c.rect.Min.Y -= arrowHeadExtraPixels
	c.rect.Max.X += arrowHeadExtraPixels
	c.rect.Max.Y += arrowHeadExtraPixels

	// 计算矢量差
	delta := c.To.Sub(c.From)
	vector := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
	// Sqrt(p*p + q*q) 求出矢量的长度
	c.vectorLength = vector.Len()
	// 转化为单位矢量，x y 都整除长度
	direction := vector.Mul(1.0 / c.vectorLength)
	angle := math.Atan2(direction.Y(), direction.X())
	glog.V(2).Infof("SetPoints(from=%v, to=%v): delta=%v, length=%.0f, angle=%5.1f",
		from, to, delta, c.vectorLength, mgl64.RadToDeg(angle))

	c.rebaseMatrix = mgl64.HomogRotate2D(-angle)
	c.rebaseMatrix = c.rebaseMatrix.Mul3(
		mgl64.Translate2D(float64(-c.From.X), float64(-c.From.Y)))
}

var (
	Yellow = color.RGBA{R: 255, G: 255, A: 255}
	Green  = color.RGBA{R: 80, G: 255, A: 80}
)

// at is the function given to the filterImage object.
func (c *Arrow) at(x, y int, under color.Color) color.Color {
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
	// arrowHeadLengthFactor*c.Thickness 是预留给剪头的长度
	if homogPoint.X() < c.vectorLength-arrowHeadLengthFactor*c.Thickness {
		if math.Abs(homogPoint.Y()) < c.Thickness/2 {
			return c.Color
		}
	} else {
		// 绘制剪头的地方，上面已经给剪头预留长度了，这里就按照剪头的长度绘制x轴和y轴，返回color的都会绘制成当先选中的颜色
		// c.vectorLength-homogPoint.X() 的长度就是直线结束，剪头开始的地方的长度
		if math.Abs(homogPoint.Y()) < (c.vectorLength-homogPoint.X())*arrowHeadWidthFactor/arrowHeadLengthFactor/2.0 {
			return c.Color
		}
	}
	return under
}

// Apply implements the ImageFilter interface.
func (c *Arrow) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
