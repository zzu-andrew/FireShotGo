package filters

import (
	"image"
	"image/color"
)

type Rectangle struct {
	// 用来表示矩形框
	Rect image.Rectangle

	// Color of the Rectangle to be drawn.
	Color color.Color

	// Thickness 指定矩形边框的宽度
	Thickness float64

	// 计算出内部矩形大小
	rectInside image.Rectangle
}

// NewRectangle creates a new Rectangle (or ellipsis) filter. It draws
// an ellipsis whose dimensions fit the given rectangle.
// You must specify the color and the thickness of the Rectangle to be drawn.
func NewRectangle(rect image.Rectangle, color color.Color, thickness float64) *Rectangle {
	c := &Rectangle{Color: color, Thickness: thickness}
	c.Rect = rect
	return c
}

func (c *Rectangle) SetRect(rect image.Rectangle) {
	c.Rect = rect

	dThickness := (int)(2 * c.Thickness)

	center := c.Rect.Min.Add(c.Rect.Max).Div(2)
	if c.Rect.Dx() > dThickness {
		c.rectInside.Min.X = c.Rect.Min.X + (int)(c.Thickness)
		c.rectInside.Max.X = c.Rect.Max.X - (int)(c.Thickness)
	} else {
		c.rectInside.Min.X = center.X
		c.rectInside.Max.X = center.X
	}

	if c.Rect.Dy() > dThickness {
		c.rectInside.Min.Y = c.Rect.Min.Y + (int)(c.Thickness)
		c.rectInside.Max.Y = c.Rect.Max.Y - (int)(c.Thickness)
	} else {
		c.rectInside.Min.Y = center.Y
		c.rectInside.Max.Y = center.Y
	}

}

// at is the function given to the filterImage object.
func (c *Rectangle) at(x, y int, under color.Color) color.Color {
	if x > c.Rect.Max.X || x < c.Rect.Min.X || y > c.Rect.Max.Y || y < c.Rect.Min.Y {
		return under
	}

	if x > c.rectInside.Min.X && x < c.rectInside.Max.X && y > c.rectInside.Min.Y && y < c.rectInside.Max.Y {
		return under
	}

	return c.Color
}

// Apply implements the ImageFilter interface.
func (c *Rectangle) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
