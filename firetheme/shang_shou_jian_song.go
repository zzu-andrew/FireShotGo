package firetheme

import (
	_ "embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

// ShanGShouJianSongTheme 设置自定义主题，主要用于支持中文
type ShanGShouJianSongTheme struct {
	RefThemeApp fyne.App
	FireFontSizeName string
}

// ShangShouJianSongXianXiTi 1. 第一种方式
// 这个功能只有go 1.16之后的版本才支持的，如果你的版本是1.16之前，请使用
// fyne bundle ShangShouJianSongXianXiTi-2.ttf > bundle.go
// 2. 第二种方式
//go:embed ShangShouJianSongXianXiTi-2.ttf
var ShangShouJianSongXianXiTi []byte

var resourceShangShouJianSongXianXiTi2Ttf = &fyne.StaticResource{
	StaticName:    "ShangShouJianSongXianXiTi-2.ttf",
	StaticContent: ShangShouJianSongXianXiTi,
}

// Font 返回的就是字体名
func (sm *ShanGShouJianSongTheme) Font(s fyne.TextStyle) fyne.Resource {

	// 因为在裁剪的主题中指定了font这里不能再对类型判断，全部按照中文显示
	//if s.Monospace || s.Bold ||  s.Italic {
	//	return theme.DefaultTheme().Font(s)
	//}

	return resourceShangShouJianSongXianXiTi2Ttf
}

func (*ShanGShouJianSongTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(n, v)
}

func (*ShanGShouJianSongTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (sm *ShanGShouJianSongTheme) Size(n fyne.ThemeSizeName) float32 {

	fs := sm.RefThemeApp.Preferences().Int(sm.FireFontSizeName)
	if fs == 0 {
		fs = 12
	}

	if n == theme.SizeNameText {
		return float32(fs)
	}

	return theme.DefaultTheme().Size(n)
}
