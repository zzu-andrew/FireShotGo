package cloud

import (
	osbytes "bytes"
	"context"
	"github.com/golang/glog"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/sms/bytes"
	"github.com/qiniu/go-sdk/v7/storage"
	"image"
	"image/png"
)

// MyPutRet 自定义返回值结构体
type MyPutRet struct {
	Key    string
	Hash   string
	Fsize  int
	Bucket string
	Name   string
}

// QiNiuManager 七牛云存储管理器
type QiNiuManager struct {
	AccessKey string
	SecretKey string
	Bucket    string
}

// NewQiNiu 创建七牛云管理对象
func NewQiNiu(accesskey string, secretKey string, bucket string) (*QiNiuManager, error) {
	// 创建七牛管理对象
	m := &QiNiuManager{
		AccessKey: accesskey,
		SecretKey: secretKey,
		Bucket:    bucket,
	}
	return m, nil
}

// QiNiuShareImage 将图片发送到七牛云上，需要传入图片名图片内容，目前仅支持网络浏览友好的png后期有需要可以扩展
// Beta版本仅支持上传华东地区，其他地区上传有点慢，杭州或者上海这边的上传速度会快一些
func (qiNiuManager *QiNiuManager) QiNiuShareImage(filename string, img image.Image) error {

	// bucket 就是个人空间下创建的文件夹
	putPolicy := storage.PutPolicy{
		Scope: qiNiuManager.Bucket,
	}

	mac := qbox.NewMac(qiNiuManager.AccessKey, qiNiuManager.SecretKey)
	upToken := putPolicy.UploadToken(mac)

	cfg := storage.Config{}
	// 空间对应的机房
	cfg.Zone = &storage.ZoneHuadong
	// 是否使用https域名
	cfg.UseHTTPS = true
	// 上传是否使用CDN上传加速
	cfg.UseCdnDomains = false

	// 构建表单上传的对象
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	// 可选配置
	putExtra := storage.PutExtra{
		Params: map[string]string{
			"x:name": "github logo",
		},
	}

	// Create PNG content of the image.
	var contentBuffer osbytes.Buffer
	_ = png.Encode(&contentBuffer, img)
	content := contentBuffer.Bytes()

	dataLen := int64(len(content))

	// key文件名称
	err := formUploader.Put(context.Background(), &ret, upToken, filename, bytes.NewReader(content), dataLen, &putExtra)
	if err != nil {
		glog.V(2).Infoln(err.Error())
		return err
	}
	//fmt.Println(filename)
	// 记录截取图片的信息
	glog.V(2).Infoln(ret, filename)
	return nil
}
