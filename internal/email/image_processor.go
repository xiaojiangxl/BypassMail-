package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器
	_ "image/jpeg" // 注册 JPEG 解码器
	"image/png"
	_ "image/png" // 注册 PNG 解码器
	"os"
)

// EmbedImageAsBase64 读取指定路径的图片文件，将其转换为PNG格式，
// 然后编码为Base64字符串，用于直接嵌入HTML的<img>标签。
func EmbedImageAsBase64(imagePath string) (string, error) {
	// 1. 读取文件
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("无法打开图片文件 '%s': %w", imagePath, err)
	}
	defer file.Close()

	// 2. 解码图片 (自动识别格式)
	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("无法解码图片 '%s': %w", imagePath, err)
	}

	// 3. 将图片编码为PNG格式到内存缓冲区
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return "", fmt.Errorf("无法将图片编码为PNG格式: %w", err)
	}

	// 4. 将PNG数据进行Base64编码
	encodedStr := base64.StdEncoding.EncodeToString(buf.Bytes())

	// 5. 格式化为Data URI
	return "data:image/png;base64," + encodedStr, nil
}
