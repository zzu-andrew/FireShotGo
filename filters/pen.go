package filters

import (
	"github.com/go-gl/mathgl/mgl64"
	"image"
	"image/color"
	"math"
	"sync"
)

type Pen struct {
	sliceLock sync.Mutex
	points    []image.Point

	// Color 指定虚线的颜色.
	Color color.Color

	// Thickness 指定虚线的宽度
	Thickness float64

	i int
}

// NewPen 创建一个新的虚线，接口中必须传入虚线的宽度、颜色以及起点.
func NewPen(to image.Point, color color.Color, thickness float64) *Pen {
	c := &Pen{Color: color, Thickness: thickness}
	c.points = make([]image.Point, 0)
	c.SetPoints(to)
	return c
}

// SetPoints 虚线打点
func (c *Pen) SetPoints(to image.Point) {

	// 每次新数据插入，如果和老数据之间隔比较大那么久按照平均值的方式进行数据插入
	thickness := c.Thickness
	if thickness <= 1 {
		thickness = 1
	}
	c.sliceLock.Lock()
	defer c.sliceLock.Unlock()

	//c.points = append(c.points, to)
	//c.i ++
	//return

	length := len(c.points)
	if length != 0 {
		from := c.points[length-1]

		delta := to.Sub(from)
		preX := 1
		if delta.X < 0 {
			preX = -1
		}

		preY := 1
		if delta.Y < 0 {
			preY = -1
		}

		deltaX := math.Abs(float64(delta.X))
		deltaY := math.Abs(float64(delta.Y))

		maxDeltas := deltaX
		if deltaX < deltaY {
			maxDeltas = deltaY
		}

		deltaSize := maxDeltas / thickness

		if deltaSize == 0 {
			return
		}

		for i := 1; i < (int)(deltaSize)-1; i++ {
			point := image.Point{X: from.X + (int)((float64)(i)*(float64)(preX)*(deltaX)/deltaSize),
				Y: from.Y + (int)((float64)(i)*(float64)(preY)*(deltaY)/deltaSize)}
			c.points = append(c.points, point)
		}

	}

	c.points = append(c.points, to)
}

func (c *Pen) getColor(x, y int) bool {
	c.sliceLock.Lock()
	defer c.sliceLock.Unlock()

	point := image.Point{X: x, Y: y}
	thi := c.Thickness * 2
	for _, v := range c.points {
		delta := v.Sub(point)
		vector := mgl64.Vec2{float64(delta.X), float64(delta.Y)}
		// 求出当前直线的长度
		if vector.Len() < thi {
			return true
		}
	}
	return false
}

// at is the function given to the filterImage object.
// under 是当前背景图片上的当前颜色
func (c *Pen) at(x, y int, under color.Color) color.Color {
	if c.getColor(x, y) {
		return c.Color
	}
	return under
}

// Apply 接口ImageFilter的实现.
// 实现方式，若是需要绘制的图，就替换为当先选中的颜色，若是不是就返回背景颜色 under
func (c *Pen) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
