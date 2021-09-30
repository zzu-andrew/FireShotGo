package screenshot

import (
	"fmt"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/golang/glog"
	"strconv"
)

type FireShotFont struct {
	fireShotFontDialog dialog.Dialog
	fireShotGoFontSize float32
}

const FireShotFontSize = "FireShotFontSize"

func (ff *FireShotFont) GetFireFontSize(gs *FireShotGO) float32 {
	fe := gs.App.Preferences().Int(FireShotFontSize)
	// 若是没有设置就是用12号字体
	if fe == 0 {
		fe = 12
	}
	return float32(fe)
}

func (ff *FireShotFont) FireShotFontEdit(gs *FireShotGO) {
	if ff.fireShotFontDialog == nil {
		// 这里弹出一个选择窗口
		widget.NewTextGrid()
		fontEntry := widget.NewEntry()
		fontEntry.Validator = validation.NewRegexp(`[1-36]`, "1 to 36")
		fe := gs.App.Preferences().Int(FireShotFontSize)
		// 若是没有设置就是用12号字体
		if fe == 0 {
			fe = 12
		}
		// 设置预写字段
		fontEntry.SetText(strconv.FormatInt(int64(fe), 10))
		// 设置占位符，虽然这里自己只有两个屏幕但是为了避免有很多屏幕的情况，还是选择使用10进制
		fontEntry.SetPlaceHolder(strconv.FormatInt(int64(fe), 10))

		// 创建新表单，点击文件-->延时截屏，弹出来该表单
		// 表单也改用中文，不再使用官方默认的英文
		ff.fireShotFontDialog = dialog.NewForm(
			"设置字体",
			"确认", "取消",
			[]*widget.FormItem{
				widget.NewFormItem("输入字体大小 ",
					fontEntry),
				widget.NewFormItem("", widget.NewLabel("输入之后需要重启生效，目前不支持动态更改！")),
			},
			func(ok bool) {
				if ok {
					// 获取并处理屏幕选择信息
					fn, err := strconv.ParseInt(fontEntry.Text, 10, 64)
					if err != nil {
						// 如果出错状态栏显示错误，状态栏目前挡放到了左下角，后期会调整到右下角
						gs.GetStatusHandle().SetText(fmt.Sprintf("Can't parse screen no in sm from %q: %s",
							fontEntry.Text, err))
						glog.Errorf("Can't parse screen no in sm from %q: %s",
							fontEntry.Text, err)
						return
					}
					gs.App.Preferences().SetInt(FireShotFontSize, int(fn))
					// 变量设置好之后只需要刷新就行了，主题里面会自动更新Font的
				}
			}, gs.Win)
	}

	size := gs.Win.Canvas().Size()
	size.Width *= 0.90
	size.Height *= 0.90
	ff.fireShotFontDialog.Resize(size)
	ff.fireShotFontDialog.Show()

}
