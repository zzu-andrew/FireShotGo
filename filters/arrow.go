package filters

import (
	"github.com/golang/glog"
	"image"
	"image/color"
	"math"

	"github.com/go-gl/mathgl/mgl64"
)

type Arrow struct {
	// From, To implement the starting and final point of the arrow:
	// arrow is pointing to the "To" direction.
	From, To image.Point

	// Color of the Arrow to be drawn.
	Color color.Color

	// Thickness of the Arrow to be drawn.
	Thickness float64

	// Rectangle enclosing arrow.
	rect image.Rectangle

	rebaseMatrix mgl64.Mat3
	vectorLength float64
}

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
	if to.X == from.X && to.Y == from.Y {
		to.X += 1 // So that arrow is always at least 1 in size.
	}
	c.From, c.To = from, to
	c.rect = image.Rectangle{Min: from, Max: to}.Canon()
	arrowHeadExtraPixels := int(arrowHeadWidthFactor*c.Thickness + 0.99)
	c.rect.Min.X -= arrowHeadExtraPixels
	c.rect.Min.Y -= arrowHeadExtraPixels
	c.rect.Max.X += arrowHeadExtraPixels
	c.rect.Max.Y += arrowHeadExtraPixels

	// 计算矢量差
	delta := c.To.Sub(c.From)
	vector := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
	c.vectorLength = vector.Len()
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
