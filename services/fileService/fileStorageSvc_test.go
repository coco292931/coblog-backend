package fileService

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"coblog-backend/common/exception"
)

// #####FIXTURE#####

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// opaqueRGBA 全不透明的图像
func opaqueRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7), uint8(y * 5), 128, 255})
		}
	}
	return img
}

// chunkBytes 拼一个 RIFF 子块（载荷奇数长度补 1 字节）
func chunkBytes(id string, body []byte) []byte {
	out := []byte(id)
	size := uint32(len(body))
	out = append(out, byte(size), byte(size>>8), byte(size>>16), byte(size>>24))
	out = append(out, body...)
	if len(body)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// riffContainer 把子块包成完整的 RIFF/WEBP 容器
func riffContainer(parts ...[]byte) []byte {
	body := bytes.Join(parts, nil)
	out := []byte("RIFF\x00\x00\x00\x00WEBP")
	binary.LittleEndian.PutUint32(out[4:8], uint32(4+len(body)))
	return append(out, body...)
}

// #####TEST#####

// 类型判定必须看内容而不是文件名后缀
func TestImageTypeByContent(t *testing.T) {
	staticWebP := riffContainer(chunkBytes("VP8 ", []byte("fake")))

	cases := []struct {
		name string
		data []byte
		want string
		ok   bool
	}{
		{"jpeg", encodeJPEG(t, opaqueRGBA(4, 4)), ".jpg", true},
		{"png", encodePNG(t, opaqueRGBA(4, 4)), ".png", true},
		{"webp", staticWebP, ".webp", true},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "", false},
		{"html", []byte("<html><body>x</body></html>"), "", false},
		{"empty", nil, "", false},
	}
	for _, c := range cases {
		got, ok := imageTypeByContent(c.data)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: 得到 (%q, %v)，期望 (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestHasAlpha(t *testing.T) {
	if hasAlpha(opaqueRGBA(4, 4)) {
		t.Error("全不透明图像不应判定为有透明像素")
	}

	transparent := opaqueRGBA(4, 4)
	transparent.Set(1, 1, color.RGBA{0, 0, 0, 0})
	if !hasAlpha(transparent) {
		t.Error("含透明像素的图像应判定为有透明像素")
	}

	// 无 alpha 通道的颜色模型直接返回 false，不必逐像素扫
	if hasAlpha(image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio420)) {
		t.Error("YCbCr 不含 alpha，不应判定为有透明像素")
	}
}

// 缩放后再判断 alpha 会误判（Resize 产物是 NRGBA），所以判断必须在缩放前
func TestCompressImageChoosesFormatByTransparency(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		// 需要缩放（400 > 100），若判断放在缩放后会误判成 png
		{"不透明大图", encodePNG(t, opaqueRGBA(400, 200)), ".jpg"},
		{"透明大图", transparentPNG(t, 400, 200), ".png"},
		{"不透明 jpeg", encodeJPEG(t, opaqueRGBA(400, 200)), ".jpg"},
		{"带透明的 gif", transparentGIF(t), ".png"},
	}
	for _, c := range cases {
		out, ext, err := compressImage(c.data, 100, 0, 75)
		if err != nil {
			t.Fatalf("%s: 压缩失败 %v", c.name, err)
		}
		if ext != c.want {
			t.Errorf("%s: 得到后缀 %q，期望 %q", c.name, ext, c.want)
		}
		if len(out) == 0 {
			t.Errorf("%s: 压缩结果为空", c.name)
		}
		if c.want == ".jpg" {
			if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
				t.Errorf("%s: 输出不是可解码的 JPEG: %v", c.name, err)
			}
		}
	}
}

// 宽高上限都要生效，且不放大
func TestResizeToFit(t *testing.T) {
	tall := opaqueRGBA(800, 5000)
	got := resizeToFit(tall, 1920, 3840)
	if got.Bounds().Dy() > 3840 || got.Bounds().Dx() > 1920 {
		t.Errorf("长图未被限制到上限内: %v", got.Bounds())
	}
	if got.Bounds().Dy() != 3840 {
		t.Errorf("长图高度 = %d，期望 3840", got.Bounds().Dy())
	}

	wide := opaqueRGBA(4000, 1000)
	got = resizeToFit(wide, 1920, 3840)
	if got.Bounds().Dx() != 1920 || got.Bounds().Dy() != 480 {
		t.Errorf("宽图尺寸 = %v，期望 1920x480", got.Bounds())
	}

	small := opaqueRGBA(100, 100)
	if got := resizeToFit(small, 1920, 3840); got.Bounds() != small.Bounds() {
		t.Errorf("小图不该被放大: %v", got.Bounds())
	}

	// 上限为 0 表示不限
	if got := resizeToFit(wide, 0, 0); got.Bounds() != wide.Bounds() {
		t.Errorf("上限为 0 时不该缩放: %v", got.Bounds())
	}
}

func TestWorthKeeping(t *testing.T) {
	if !worthKeeping(1000, 949) {
		t.Error("省下 5% 以上应保留压缩图")
	}
	if worthKeeping(1000, 950) {
		t.Error("只省 5% 不该单独存一份")
	}
	if worthKeeping(1000, 1200) {
		t.Error("压完更大不该保留压缩图")
	}
}

func TestCheckPixels(t *testing.T) {
	data := encodePNG(t, opaqueRGBA(200, 200)) // 40000 像素

	if err := checkPixels(data, 39999); !errors.Is(err, exception.ApiImageTooManyPixels) {
		t.Errorf("超限应返回像素过多错误，得到 %v", err)
	}
	if err := checkPixels(data, 40000); err != nil {
		t.Errorf("刚好等于上限不该拦，得到 %v", err)
	}
	if err := checkPixels(data, 0); err != nil {
		t.Errorf("上限为 0 表示不限制，得到 %v", err)
	}
	if err := checkPixels([]byte("not an image"), 10); err != nil {
		t.Errorf("读不出头部时不该拦，得到 %v", err)
	}
}

// 压缩图要按 EXIF 方向转正，否则竖拍照片的缩略图会躺倒
func TestDecodeImageAppliesExifOrientation(t *testing.T) {
	data := jpegWithOrientation(t, opaqueRGBA(4, 2), 6)

	img, err := decodeImage(data)
	if err != nil {
		t.Fatal(err)
	}
	// Orientation=6 → 顺时针转 90°，宽高互换
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 4 {
		t.Errorf("尺寸 = %v，期望 2x4（已按方向转正）", img.Bounds())
	}
}

// 动图 webp 取首帧：剥出 ANMF 里的位流重新包成最小容器
func TestFirstWebPFrame(t *testing.T) {
	payload := []byte("vp8l-bitstream") // 本用例只校验容器结构，不真解码

	t.Run("无损位流", func(t *testing.T) {
		frame, ok := firstWebPFrame(animatedWebP(t, chunkBytes("VP8L", payload)))
		if !ok {
			t.Fatal("应能剥出首帧")
		}
		chunks, ok := webpTopChunks(frame)
		if !ok || len(chunks) != 1 {
			t.Fatalf("首帧容器应只含一个子块，得到 %d 个", len(chunks))
		}
		if chunks[0].id != "VP8L" || !bytes.Equal(chunks[0].body, payload) {
			t.Errorf("子块 = %s(%q)，期望 VP8L(%q)", chunks[0].id, chunks[0].body, payload)
		}
		// RIFF 长度字段必须是文件长度 -8，x/image 的 riff 读取器依赖它
		if got, want := binary.LittleEndian.Uint32(frame[4:8]), uint32(len(frame)-8); got != want {
			t.Errorf("RIFF 长度 = %d，期望 %d", got, want)
		}
	})

	t.Run("带独立 alpha 的位流", func(t *testing.T) {
		// 载荷取奇数长度，顺带覆盖「子块按偶数对齐」的处理
		frame, ok := firstWebPFrame(animatedWebP(t, chunkBytes("ALPH", []byte{0, 1, 2}), chunkBytes("VP8 ", payload)))
		if !ok {
			t.Fatal("应能剥出首帧")
		}
		chunks, ok := webpTopChunks(frame)
		if !ok || len(chunks) != 3 {
			t.Fatalf("期望 VP8X+ALPH+VP8 三个子块，得到 %d 个", len(chunks))
		}
		// 没有 VP8X 声明 alpha，解码器会拒绝后面的 ALPH
		if chunks[0].id != "VP8X" || chunks[0].body[0] != webpAlphaFlags {
			t.Errorf("子块 0 = %s，期望声明 alpha 的 VP8X", chunks[0].id)
		}
		if chunks[1].id != "ALPH" || chunks[2].id != "VP8 " {
			t.Errorf("子块顺序 = %s,%s，期望 ALPH,VP8", chunks[1].id, chunks[2].id)
		}
	})

	t.Run("非动图返回 false", func(t *testing.T) {
		static := riffContainer(chunkBytes("VP8L", payload))
		if _, ok := firstWebPFrame(static); ok {
			t.Error("静态 webp 不该走首帧剥离")
		}
	})

	t.Run("读取首帧尺寸", func(t *testing.T) {
		w, h, ok := webpFirstFrameSize(animatedWebP(t, chunkBytes("VP8L", payload)))
		if !ok || w != 16 || h != 8 {
			t.Errorf("尺寸 = %dx%d(%v)，期望 16x8", w, h, ok)
		}
	})
}

// #####HELPER#####

func transparentPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := opaqueRGBA(w, h)
	img.Set(0, 0, color.RGBA{0, 0, 0, 0})
	return encodePNG(t, img)
}

func transparentGIF(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 0, 0}}
	p := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for i := range p.Pix {
		p.Pix[i] = uint8(i % len(pal))
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, p, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// animatedWebP 拼一个结构合法的动图 webp，帧头尺寸为 16x8
func animatedWebP(t *testing.T, frameChunks ...[]byte) []byte {
	t.Helper()
	anmf := make([]byte, anmfHeaderLen)
	anmf[6], anmf[7], anmf[8] = 0x0f, 0x00, 0x00   // 宽-1 = 15
	anmf[9], anmf[10], anmf[11] = 0x07, 0x00, 0x00 // 高-1 = 7
	anmf = append(anmf, bytes.Join(frameChunks, nil)...)

	vp8x := []byte{0x02, 0, 0, 0, 0x0f, 0, 0, 0x07, 0, 0} // 动画标志 + 画布尺寸
	return riffContainer(
		chunkBytes("VP8X", vp8x),
		chunkBytes("ANIM", []byte{0, 0, 0, 0, 0, 0}),
		chunkBytes("ANMF", anmf),
	)
}

// jpegWithOrientation 在 JPEG 的 SOI 之后插入一个只含 Orientation 的 EXIF 段
func jpegWithOrientation(t *testing.T, img image.Image, orientation uint16) []byte {
	t.Helper()
	raw := encodeJPEG(t, img)

	tiff := []byte{
		'I', 'I', 0x2A, 0x00, // 小端 TIFF 头
		0x08, 0x00, 0x00, 0x00, // IFD0 偏移
		0x01, 0x00, // 一个条目
		0x12, 0x01, // tag 0x0112 Orientation
		0x03, 0x00, // type SHORT
		0x01, 0x00, 0x00, 0x00, // count 1
		byte(orientation), 0x00, 0x00, 0x00, // 值
		0x00, 0x00, 0x00, 0x00, // 无下一个 IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	size := len(payload) + 2
	app1 := []byte{0xFF, 0xE1, byte(size >> 8), byte(size)}

	out := append([]byte{}, raw[:2]...) // SOI
	out = append(out, app1...)
	out = append(out, payload...)
	return append(out, raw[2:]...)
}
