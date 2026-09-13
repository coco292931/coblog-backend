package fileService

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"sort"
	"testing"
)

// #####FIXTURE：拼装带各种元数据的图片（不复用生产代码，避免自证）#####

type testEntry struct {
	tag uint16
	typ uint16
	val []byte
}

func testTypeSize(typ uint16) int {
	switch typ {
	case 3:
		return 2
	case 4:
		return 4
	case 5:
		return 8
	}
	return 1 // BYTE / ASCII / UNDEFINED
}

func entry(tag, typ uint16, val []byte) testEntry {
	return testEntry{tag: tag, typ: typ, val: val}
}

func ascii(s string) []byte {
	return append([]byte(s), 0) // ASCII 类型要以 NUL 结尾
}

func short(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func rational(num, den uint32) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], num)
	binary.LittleEndian.PutUint32(b[4:8], den)
	return b
}

// buildTestExif 拼一个小端 TIFF（IFD0 + Exif IFD + GPS IFD），偏移按实际布局算好
func buildTestExif(ifd0, exif, gps []testEntry) []byte {
	return buildTestExifBO(binary.LittleEndian, ifd0, exif, gps)
}

func buildTestExifBO(bo binary.ByteOrder, ifd0, exif, gps []testEntry) []byte {
	ents0 := append([]testEntry{}, ifd0...)
	if len(exif) > 0 {
		ents0 = append(ents0, entry(0x8769, 4, make([]byte, 4))) // ExifOffset
	}
	if len(gps) > 0 {
		ents0 = append(ents0, entry(0x8825, 4, make([]byte, 4))) // GPSInfo
	}
	sortEntries(ents0)
	sortEntries(exif)
	sortEntries(gps)

	size0 := 2 + 12*len(ents0) + 4
	exifOff := 8 + size0
	sizeExif := 0
	if len(exif) > 0 {
		sizeExif = 2 + 12*len(exif) + 4
	}
	gpsOff := exifOff + sizeExif
	sizeGps := 0
	if len(gps) > 0 {
		sizeGps = 2 + 12*len(gps) + 4
	}
	dataOff := gpsOff + sizeGps

	for i := range ents0 {
		switch ents0[i].tag {
		case 0x8769:
			bo.PutUint32(ents0[i].val, uint32(exifOff))
		case 0x8825:
			bo.PutUint32(ents0[i].val, uint32(gpsOff))
		}
	}

	out := make([]byte, 8)
	if bo == binary.LittleEndian {
		copy(out, "II")
	} else {
		copy(out, "MM")
	}
	bo.PutUint16(out[2:4], 42)
	bo.PutUint32(out[4:8], 8)

	var data []byte
	write := func(entries []testEntry) {
		head := make([]byte, 2)
		bo.PutUint16(head, uint16(len(entries)))
		out = append(out, head...)
		for _, e := range entries {
			raw := make([]byte, 12)
			bo.PutUint16(raw[0:2], e.tag)
			bo.PutUint16(raw[2:4], e.typ)
			bo.PutUint32(raw[4:8], uint32(len(e.val)/testTypeSize(e.typ)))
			if len(e.val) <= 4 {
				copy(raw[8:12], e.val)
			} else {
				bo.PutUint32(raw[8:12], uint32(dataOff+len(data)))
				data = append(data, e.val...)
			}
			out = append(out, raw...)
		}
		out = append(out, 0, 0, 0, 0)
	}
	write(ents0)
	if len(exif) > 0 {
		write(exif)
	}
	if len(gps) > 0 {
		write(gps)
	}
	return append(out, data...)
}

func sortEntries(entries []testEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].tag < entries[j].tag })
}

func jpegSegment(marker byte, payload []byte) []byte {
	size := len(payload) + 2
	return append([]byte{0xFF, marker, byte(size >> 8), byte(size)}, payload...)
}

func app1Exif(tiff []byte) []byte {
	return jpegSegment(0xE1, append([]byte("Exif\x00\x00"), tiff...))
}

// jpegWithSegments 在 SOI 之后插入给定段
func jpegWithSegments(t *testing.T, img image.Image, segments ...[]byte) []byte {
	t.Helper()
	raw := encodeJPEG(t, img)
	out := append([]byte{}, raw[:2]...)
	for _, seg := range segments {
		out = append(out, seg...)
	}
	return append(out, raw[2:]...)
}

// jpegTailAfterSOS 取 SOS 之后的压缩数据（用来证明像素数据一个字节都没动）
func jpegTailAfterSOS(data []byte) []byte {
	for i := 2; i+1 < len(data); i++ {
		if data[i] == 0xFF && data[i+1] == 0xDA {
			return data[i:]
		}
	}
	return nil
}

// dumpExifTags 独立解析重建后的 TIFF，返回 IFD0 与 Exif IFD 的 tag → 值字节
func dumpExifTags(t *testing.T, tiff []byte) map[uint16][]byte {
	t.Helper()
	if len(tiff) < 8 || string(tiff[0:2]) != "II" {
		t.Fatalf("期望小端 TIFF，实际 %q", tiff)
	}
	tags := map[uint16][]byte{}
	collect := func(off int) uint32 {
		n := int(binary.LittleEndian.Uint16(tiff[off : off+2]))
		for i := 0; i < n; i++ {
			raw := tiff[off+2+i*12 : off+2+i*12+12]
			tag := binary.LittleEndian.Uint16(raw[0:2])
			typ := binary.LittleEndian.Uint16(raw[2:4])
			size := int(binary.LittleEndian.Uint32(raw[4:8])) * testTypeSize(typ)
			if size <= 4 {
				tags[tag] = bytes.Clone(raw[8 : 8+size])
			} else {
				dataOff := int(binary.LittleEndian.Uint32(raw[8:12]))
				if dataOff+size > len(tiff) {
					t.Fatalf("tag %#x 的数据越界", tag)
				}
				tags[tag] = bytes.Clone(tiff[dataOff : dataOff+size])
			}
		}
		return binary.LittleEndian.Uint32(tiff[off+2+n*12 : off+2+n*12+4])
	}
	if next := collect(8); next != 0 {
		t.Errorf("IFD0 不该再有下一个 IFD（缩略图），实际偏移 %d", next)
	}
	if ptr, ok := tags[0x8769]; ok {
		collect(int(binary.LittleEndian.Uint32(ptr)))
	}
	return tags
}

// exifPayloadOf 取出 JPEG 里重建后的 EXIF 裸 TIFF
func exifPayloadOf(t *testing.T, jpeg []byte) []byte {
	t.Helper()
	for off := 2; off+4 <= len(jpeg); {
		if jpeg[off] != 0xFF {
			t.Fatalf("段结构异常，偏移 %d", off)
		}
		size := int(binary.BigEndian.Uint16(jpeg[off+2 : off+4]))
		payload := jpeg[off+4 : off+2+size]
		if jpeg[off+1] == 0xE1 && bytes.HasPrefix(payload, exifHeader) {
			return payload[len(exifHeader):]
		}
		if jpeg[off+1] == 0xDA {
			break
		}
		off += 2 + size
	}
	t.Fatal("重建后的 JPEG 里没有 EXIF 段")
	return nil
}

func pngChunks(data []byte) map[string][]byte {
	chunks := map[string][]byte{}
	for off := 8; off+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])
		chunks[typ] = data[off+8 : off+8+length]
		off += 12 + length
	}
	return chunks
}

func pngChunkRaw(typ string, body []byte) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint32(out[0:4], uint32(len(body)))
	copy(out[4:8], typ)
	out = append(out, body...)
	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(append([]byte(typ), body...)))
	return append(out, crc...)
}

func webpChunk(id string, body []byte) []byte {
	return chunkBytes(id, body)
}

// #####TEST#####

// 原图：去掉 GPS / 文本类元数据，保留作者、版权、机身镜头与拍摄参数
func TestStripJPEGMetadataKeepsCameraDropsLocation(t *testing.T) {
	photo := opaqueRGBA(8, 8)
	tiff := buildTestExif(
		[]testEntry{
			entry(0x010F, 2, ascii("TestMake")),
			entry(0x0110, 2, ascii("TestModel")),
			entry(0x0112, 3, short(6)), // Orientation：像素不动，必须留着
			entry(0x0132, 2, ascii("2026:09:13 10:00:00")),
			entry(0x013B, 2, ascii("Coco")),
			entry(0x8298, 2, ascii("(c) Coco")),
			entry(0x010E, 2, ascii("Taken at home")),  // ImageDescription：可能写地点
			entry(0x0131, 2, ascii("SecretSoft 1.0")), // Software
		},
		[]testEntry{
			entry(0x829A, 5, rational(1, 125)),
			entry(0x829D, 5, rational(28, 10)),
			entry(0x8827, 3, short(200)),
			entry(0x9003, 2, ascii("2026:09:13 10:00:00")),
			entry(0x9010, 2, ascii("+08:00")), // OffsetTime：时区能透地点，不保留
			entry(0x920A, 5, rational(50, 1)),
			entry(0xA431, 2, ascii("body-serial-123")), // BodySerialNumber
			entry(0xA434, 2, ascii("TestLens")),
			entry(0xA435, 2, ascii("lens-serial-456")),    // LensSerialNumber
			entry(0x9286, 7, ascii("usercomment-secret")), // UserComment
			entry(0x927C, 7, ascii("makernote-secret")),   // MakerNote
		},
		[]testEntry{
			entry(0x0000, 1, []byte{2, 3, 0, 0}),
			entry(0x0002, 5, rational(31, 1)),  // GPSLatitude
			entry(0x0004, 5, rational(121, 1)), // GPSLongitude
		},
	)

	source := jpegWithSegments(t, photo,
		jpegSegment(0xE0, []byte("JFIF\x00")),
		app1Exif(tiff),
		jpegSegment(0xE1, append([]byte("http://ns.adobe.com/xap/1.0/\x00"), []byte("<xmp>gps-secret</xmp>")...)),
		jpegSegment(0xE2, []byte("ICCPROFILE-DATA")),
		jpegSegment(0xED, []byte("iptc-location-secret")),
		jpegSegment(0xFE, []byte("comment-secret")),
	)

	stripped := stripJPEGMetadata(source)
	if len(stripped) >= len(source) {
		t.Logf("清理后 %d 字节，原始 %d 字节", len(stripped), len(source))
	}

	// 像素数据必须一个字节都不动
	if !bytes.Equal(jpegTailAfterSOS(source), jpegTailAfterSOS(stripped)) {
		t.Fatal("SOS 之后的压缩数据被改动了")
	}

	// 文本类与可追踪类元数据必须消失
	for _, secret := range []string{
		"gps-secret", "iptc-location-secret", "comment-secret", "usercomment-secret", "makernote-secret",
		"Taken at home", "SecretSoft",
		"body-serial-123", "lens-serial-456", "+08:00", // 序列号与时间偏移
	} {
		if bytes.Contains(stripped, []byte(secret)) {
			t.Errorf("输出里仍残留 %q", secret)
		}
	}
	if !bytes.Contains(stripped, []byte("ICCPROFILE-DATA")) {
		t.Error("ICC 配置被误删")
	}

	// 白名单标签要原样保留
	tags := dumpExifTags(t, exifPayloadOf(t, stripped))
	want := map[uint16][]byte{
		0x010F: ascii("TestMake"),
		0x0110: ascii("TestModel"),
		0x0112: short(6),
		0x013B: ascii("Coco"),
		0x8298: ascii("(c) Coco"),
		0x829A: rational(1, 125),
		0x829D: rational(28, 10),
		0x8827: short(200),
		0x920A: rational(50, 1),
		0xA434: ascii("TestLens"),
	}
	for tag, val := range want {
		got, ok := tags[tag]
		if !ok {
			t.Errorf("tag %#x 丢失", tag)
			continue
		}
		if !bytes.Equal(got, val) {
			t.Errorf("tag %#x = %v，期望 %v", tag, got, val)
		}
	}
	if _, ok := tags[0x8825]; ok {
		t.Error("GPSIFD 指针不应保留")
	}
	// 序列号与时区偏移不该进白名单
	for _, tag := range []uint16{0xA431, 0xA435, 0x9010, 0x9011, 0x9012} {
		if _, ok := tags[tag]; ok {
			t.Errorf("tag %#x 不该保留", tag)
		}
	}

	// 图片仍可解码，且按方向转正的行为不变
	img, err := decodeImage(stripped)
	if err != nil {
		t.Fatalf("清理后无法解码: %v", err)
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("尺寸 = %v，期望 8x8", img.Bounds())
	}
}

// EXIF 结构异常时整段丢弃，但不能把图片弄坏
func TestStripJPEGMetadataDropsBrokenExif(t *testing.T) {
	broken := buildTestExif([]testEntry{entry(0x013B, 2, ascii("Coco"))}, nil, nil)
	binary.LittleEndian.PutUint32(broken[4:8], 0xFFFF) // IFD0 偏移指到文件外

	source := jpegWithSegments(t, opaqueRGBA(8, 8), app1Exif(broken), jpegSegment(0xE2, []byte("ICCPROFILE-DATA")))
	stripped := stripJPEGMetadata(source)

	if bytes.Contains(stripped, []byte("Coco")) {
		t.Error("结构异常时不该保留 EXIF 内容")
	}
	if !bytes.Contains(stripped, []byte("ICCPROFILE-DATA")) {
		t.Error("ICC 配置不该受影响")
	}
	if !bytes.Equal(jpegTailAfterSOS(source), jpegTailAfterSOS(stripped)) {
		t.Fatal("像素数据被改动了")
	}
	if _, _, err := image.Decode(bytes.NewReader(stripped)); err != nil {
		t.Fatalf("清理后无法解码: %v", err)
	}
}

// PNG：文本块与 eXIf 丢弃，显色相关的块与像素数据保留
func TestStripPNGMetadata(t *testing.T) {
	raw := encodePNG(t, opaqueRGBA(16, 16))
	insert := bytes.Join([][]byte{
		pngChunkRaw("tEXt", []byte("Comment\x00secret-place")),
		pngChunkRaw("zTXt", []byte("Description\x00\x00secret")),
		pngChunkRaw("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00<xmp>secret</xmp>")),
		pngChunkRaw("eXIf", buildTestExif(nil, nil, []testEntry{entry(0x0002, 5, rational(31, 1))})),
		pngChunkRaw("tIME", []byte{0x07, 0xEA, 9, 13, 10, 0, 0}),
		pngChunkRaw("iCCP", []byte("profile\x00\x00fake")),
	}, nil)
	const ihdrLen = 8 + 13 + 4 // IHDR 块（含长度与 CRC），必须紧跟在签名之后
	source := append(append(append([]byte{}, raw[:8+ihdrLen]...), insert...), raw[8+ihdrLen:]...)

	stripped := stripPNGMetadata(source)
	chunks := pngChunks(stripped)
	for _, typ := range []string{"tEXt", "zTXt", "iTXt", "eXIf"} {
		if _, ok := chunks[typ]; ok {
			t.Errorf("%s 块未被丢弃", typ)
		}
	}
	for _, typ := range []string{"IHDR", "IDAT", "IEND", "tIME", "iCCP"} {
		if _, ok := chunks[typ]; !ok {
			t.Errorf("%s 块被误删", typ)
		}
	}
	if !bytes.Equal(chunks["IDAT"], pngChunks(raw)["IDAT"]) {
		t.Error("IDAT 数据被改动了")
	}
	for _, secret := range []string{"secret-place", "secret"} {
		if bytes.Contains(stripped, []byte(secret)) {
			t.Errorf("输出里仍残留 %q", secret)
		}
	}

	before, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := image.Decode(bytes.NewReader(stripped))
	if err != nil {
		t.Fatalf("清理后无法解码: %v", err)
	}
	assertSamePixels(t, before, after)
}

// webp：重建 EXIF 块、丢掉 XMP 块，并同步清掉 VP8X 里的 XMP 标志位
func TestStripWebPMetadata(t *testing.T) {
	tiff := buildTestExif(
		[]testEntry{entry(0x010F, 2, ascii("TestMake"))},
		nil,
		[]testEntry{entry(0x0002, 5, rational(31, 1))},
	)
	vp8x := []byte{0x1C, 0, 0, 0, 0x0f, 0, 0, 0x07, 0, 0} // 动画+alpha+EXIF(0x08)+XMP(0x04)
	source := riffContainer(
		webpChunk("VP8X", vp8x),
		webpChunk("EXIF", tiff),
		webpChunk("XMP ", []byte("<xmp>gps-secret</xmp>")),
		webpChunk("VP8L", []byte("bitstream")),
	)

	stripped := stripWebPMetadata(source)
	chunks, ok := webpTopChunks(stripped)
	if !ok {
		t.Fatal("清理后容器不合法")
	}
	ids := []string{}
	for _, c := range chunks {
		ids = append(ids, c.id)
	}
	if len(ids) != 3 || ids[0] != "VP8X" || ids[1] != "EXIF" || ids[2] != "VP8L" {
		t.Fatalf("子块 = %v，期望 VP8X/EXIF/VP8L", ids)
	}
	if bytes.Contains(stripped, []byte("gps-secret")) {
		t.Error("XMP 内容仍残留")
	}
	if chunks[0].body[0]&0x04 != 0 {
		t.Error("VP8X 的 XMP 标志位未清掉")
	}
	if chunks[0].body[0]&0x08 == 0 {
		t.Error("VP8X 的 EXIF 标志位不该清掉")
	}

	tags := dumpExifTags(t, chunks[1].body)
	if !bytes.Equal(tags[0x010F], ascii("TestMake")) {
		t.Error("机型信息未保留")
	}
	if _, ok := tags[0x8825]; ok {
		t.Error("GPS 指针不应保留")
	}
}

// 大端 EXIF 同样要处理（部分相机写 MM）
func TestStripJPEGMetadataBigEndian(t *testing.T) {
	bo := binary.BigEndian
	tiff := buildTestExifBO(bo,
		[]testEntry{
			entry(0x010F, 2, ascii("BigEndianCam")),
			entry(0x0112, 3, beUint16(bo, 6)),
		},
		nil,
		[]testEntry{entry(0x0002, 5, beRational(bo, 31, 1))},
	)

	stripped := stripJPEGMetadata(jpegWithSegments(t, opaqueRGBA(8, 8), app1Exif(tiff)))
	if bytes.Contains(stripped, []byte("GPS")) {
		t.Error("不该残留 GPS 相关字节")
	}

	// dumpExifTags 只认小端，这里手工看重建后的字节：字节序保持 + 机型字符串还在
	tiffOut := exifPayloadOf(t, stripped)
	if !bytes.HasPrefix(tiffOut, []byte("MM")) {
		t.Errorf("重建结果应保持大端 TIFF，实际 %q", tiffOut[:2])
	}
	if !bytes.Contains(tiffOut, ascii("BigEndianCam")) {
		t.Error("机型信息未保留")
	}
	if _, err := decodeImage(stripped); err != nil {
		t.Fatalf("清理后无法解码: %v", err)
	}
}

func beUint16(bo binary.ByteOrder, v uint16) []byte {
	b := make([]byte, 2)
	bo.PutUint16(b, v)
	return b
}

func beRational(bo binary.ByteOrder, num, den uint32) []byte {
	b := make([]byte, 8)
	bo.PutUint32(b[0:4], num)
	bo.PutUint32(b[4:8], den)
	return b
}

func assertSamePixels(t *testing.T, before, after image.Image) {
	t.Helper()
	if before.Bounds() != after.Bounds() {
		t.Fatalf("尺寸不同: %v vs %v", before.Bounds(), after.Bounds())
	}
	b := before.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if before.At(x, y) != after.At(x, y) {
				t.Fatalf("(%d,%d) 像素不同: %v vs %v", x, y, before.At(x, y), after.At(x, y))
			}
		}
	}
}
