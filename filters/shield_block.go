package filters

import (
	"image"
	"image/color"
)

type ShieldBlock struct {
	// 用来表示矩形框
	Rect image.Rectangle

	// Color of the ShieldBlock to be drawn.
	Color color.Color
}

// NewShieldBlock creates a new ShieldBlock (or ellipsis) filter. It draws
// an ellipsis whose dimensions fit the given rectangle.
// You must specify the color and the thickness of the ShieldBlock to be drawn.
func NewShieldBlock(rect image.Rectangle, color color.Color) *ShieldBlock {
	c := &ShieldBlock{Color: color}
	c.Rect = rect
	return c
}

func (c *ShieldBlock) SetRect(rect image.Rectangle) {
	c.Rect = rect
}

// at is the function given to the filterImage object.
func (c *ShieldBlock) at(x, y int, under color.Color) color.Color {
	if x > c.Rect.Max.X || x < c.Rect.Min.X || y > c.Rect.Max.Y || y < c.Rect.Min.Y {
		return under
	}
	return c.Color
}

// Apply implements the ImageFilter interface.
func (c *ShieldBlock) Apply(image image.Image) image.Image {
	return &filterImage{image, c.at}
}
