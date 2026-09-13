package fileService

import (
	"bytes"
	"encoding/binary"
	"sort"
)

// stripMetadata 落盘前去掉原图里可能带位置信息的元数据。
// 只改容器结构，像素数据一个字节都不动；相机 / 作者 / 版权 / 拍摄参数按白名单保留，
// 结构异常时宁可丢掉整段元数据也不泄漏位置。
func stripMetadata(data []byte, ext string) []byte {
	switch ext {
	case ".jpg":
		return stripJPEGMetadata(data)
	case ".png":
		return stripPNGMetadata(data)
	case ".webp":
		return stripWebPMetadata(data)
	}
	return data
}

// #####JPEG#####

const (
	jpegSOI   = 0xD8
	jpegSOS   = 0xDA
	jpegAPP0  = 0xE0 // JFIF，解码需要
	jpegAPP1  = 0xE1 // EXIF / XMP
	jpegAPP2  = 0xE2 // ICC 色彩配置、MPF
	jpegAPP13 = 0xED // IPTC / Photoshop
	jpegAPP14 = 0xEE // Adobe 色彩变换标识
	jpegCOM   = 0xFE
)

var (
	exifHeader = []byte("Exif\x00\x00")
)

// stripJPEGMetadata 重建 EXIF 段并丢掉 XMP / IPTC / 注释与厂商私有 APPn。
// 保留 APP0 / APP2 / APP14；SOS 之后的压缩数据原样拷贝。
func stripJPEGMetadata(data []byte) []byte {
	if len(data) < 4 || data[0] != 0xFF || data[1] != jpegSOI {
		return data
	}

	out := make([]byte, 0, len(data))
	out = append(out, 0xFF, jpegSOI)
	wroteExif := false

	for off := 2; off+2 <= len(data); {
		if data[off] != 0xFF {
			return data // 段结构异常，原样返回
		}
		marker := data[off+1]
		if marker == jpegSOS {
			// 其后是熵编码数据，直到文件末尾都原样拷回
			return append(out, data[off:]...)
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			out = append(out, data[off], data[off+1]) // 无载荷的独立标记
			off += 2
			continue
		}
		if off+4 > len(data) {
			return data
		}
		size := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		if size < 2 || off+2+size > len(data) {
			return data
		}
		payload := data[off+4 : off+2+size]

		switch {
		case marker == jpegAPP1 && bytes.HasPrefix(payload, exifHeader):
			if wroteExif {
				break // 重复的 EXIF 段直接丢弃
			}
			if rebuilt, ok := rebuildExif(payload[len(exifHeader):]); ok {
				wroteExif = true
				out = appendJPEGSegment(out, jpegAPP1, append(append([]byte{}, exifHeader...), rebuilt...))
			}
			// 重建失败（结构异常）→ 整段丢弃
		case marker == jpegAPP1:
			// XMP 与未知 APP1：都可能写位置，一律丢弃
		case marker == jpegCOM || marker == jpegAPP13:
			// 注释与 IPTC：丢弃
		case marker == jpegAPP0 || marker == jpegAPP2 || marker == jpegAPP14:
			out = appendJPEGSegment(out, marker, payload)
		case marker >= 0xE3 && marker <= 0xEF:
			// 厂商私有 APPn：丢弃
		default:
			out = appendJPEGSegment(out, marker, payload)
		}
		off += 2 + size
	}
	return data // 没走到 SOS，不像 JPEG，原样返回
}

// appendJPEGSegment 按「FF + 标记 + 含自身的 2 字节长度 + 载荷」追加一个段
func appendJPEGSegment(out []byte, marker byte, payload []byte) []byte {
	size := len(payload) + 2
	out = append(out, 0xFF, marker, byte(size>>8), byte(size))
	return append(out, payload...)
}

// #####PNG#####

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// stripPNGMetadata 丢弃可能写位置的文本块与 eXIf；IHDR/PLTE/IDAT/IEND 与
// 影响显色的 iCCP/sRGB/gAMA/cHRM/tRNS 等一律保留。
func stripPNGMetadata(data []byte) []byte {
	if !bytes.HasPrefix(data, pngSignature) {
		return data
	}

	out := append([]byte{}, pngSignature...)
	for off := len(pngSignature); off+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[off : off+4]))
		if length < 0 || off+12+length > len(data) {
			return data // 结构异常，原样返回
		}
		typ := string(data[off+4 : off+8])
		end := off + 12 + length // 含 4 字节 CRC

		switch typ {
		case "eXIf", "tEXt", "iTXt", "zTXt":
			// 丢弃
		default:
			out = append(out, data[off:end]...)
		}
		off = end
		if typ == "IEND" {
			break
		}
	}
	return out
}

// #####WEBP#####

const (
	webpFlagXMP  = 1 << 2
	webpFlagEXIF = 1 << 3
)

// stripWebPMetadata 重建 EXIF 块、丢弃 XMP 块，并同步清掉 VP8X 里的对应标志位。
func stripWebPMetadata(data []byte) []byte {
	chunks, ok := webpTopChunks(data)
	if !ok {
		return data
	}

	var (
		parts   []riffChunk
		vp8x    = -1
		changed bool
		newExif []byte
	)
	for _, c := range chunks {
		switch c.id {
		case "VP8X":
			vp8x = len(parts)
			parts = append(parts, c)
		case "EXIF":
			changed = true // 重建成功就换掉，失败就丢弃
			if rebuilt, ok := rebuildExif(trimExifPrefix(c.body)); ok {
				newExif = rebuilt
				parts = append(parts, riffChunk{id: "EXIF", body: rebuilt})
			}
		case "XMP ":
			changed = true
		default:
			parts = append(parts, c)
		}
	}
	if !changed {
		return data
	}

	if vp8x >= 0 {
		body := append([]byte{}, parts[vp8x].body...)
		if len(body) >= 1 {
			flags := body[0] &^ (webpFlagEXIF | webpFlagXMP)
			if len(newExif) > 0 {
				flags |= webpFlagEXIF
			}
			body[0] = flags
		}
		parts[vp8x].body = body
	}
	return packWebP(parts)
}

// trimExifPrefix 去掉部分写入方加在块首的 "Exif\0\0"
func trimExifPrefix(body []byte) []byte {
	if bytes.HasPrefix(body, exifHeader) {
		return body[len(exifHeader):]
	}
	return body
}

// #####EXIF 重建#####

const exifTagExifPointer = 0x8769

// exifKeepIFD0 与 exifKeepExif 是保留白名单：作者版权、机身镜头、拍摄参数、时间，
// 以及结构上必需的几个。不在表里的（GPS、MakerNote、UserComment、Software、
// ImageDescription、厂商私有标签…）一律不写回。
// 序列号（Body/LensSerialNumber）与 OffsetTime 时区偏移可定位到具体设备/地区，刻意不保留。
var (
	exifKeepIFD0 = map[uint16]bool{
		0x010F: true, // Make 机身厂商
		0x0110: true, // Model 机型
		0x0112: true, // Orientation 方向（像素不动，只能靠它转正）
		0x011A: true, // XResolution
		0x011B: true, // YResolution
		0x0128: true, // ResolutionUnit
		0x0132: true, // DateTime
		0x013B: true, // Artist 作者
		0x8298: true, // Copyright 版权
	}

	exifKeepExif = map[uint16]bool{
		0x829A: true, // ExposureTime 快门
		0x829D: true, // FNumber 光圈
		0x8822: true, // ExposureProgram
		0x8827: true, // ISOSpeedRatings
		0x9003: true, // DateTimeOriginal
		0x9004: true, // DateTimeDigitized
		0x9101: true, // ComponentsConfiguration
		0x9204: true, // ExposureBiasValue
		0x9207: true, // MeteringMode
		0x9209: true, // Flash
		0x920A: true, // FocalLength 焦距
		0xA001: true, // ColorSpace
		0xA002: true, // PixelXDimension
		0xA003: true, // PixelYDimension
		0xA402: true, // ExposureMode
		0xA403: true, // WhiteBalance
		0xA405: true, // FocalLengthIn35mmFilm
		0xA406: true, // SceneCaptureType
		0xA432: true, // LensSpecification 镜头规格
		0xA433: true, // LensMake
		0xA434: true, // LensModel 镜头型号
	}
)

// exifEntry 一个 IFD 条目，value 为原样拷贝的值字节（≤4 字节内联，否则指向外部数据）
type exifEntry struct {
	tag   uint16
	typ   uint16
	count uint32
	value []byte
}

// rebuildExif 解析裸 TIFF、按白名单挑出条目后重新拼装（偏移全部重算）。
// 返回 false 表示结构异常，调用方应整段丢弃 EXIF。
func rebuildExif(tiff []byte) ([]byte, bool) {
	bo, ifd0Off, ok := readTIFFHeader(tiff)
	if !ok {
		return nil, false
	}
	ifd0, ok := readIFD(tiff, bo, ifd0Off)
	if !ok {
		return nil, false
	}

	var (
		keep0    []exifEntry
		exifOff  int
		keepExif []exifEntry
	)
	for _, e := range ifd0 {
		if e.tag == exifTagExifPointer {
			exifOff = int(entryUint32(bo, e))
			continue
		}
		if exifKeepIFD0[e.tag] {
			keep0 = append(keep0, e)
		}
	}
	if exifOff > 0 {
		entries, ok := readIFD(tiff, bo, exifOff)
		if !ok {
			return nil, false
		}
		for _, e := range entries {
			if exifKeepExif[e.tag] {
				keepExif = append(keepExif, e)
			}
		}
	}
	if len(keep0) == 0 && len(keepExif) == 0 {
		return nil, false
	}
	// IFD1（内嵌缩略图）整体丢弃：无保留价值，且能省一次偏移重算
	return buildTIFF(bo, keep0, keepExif), true
}

func readTIFFHeader(tiff []byte) (binary.ByteOrder, int, bool) {
	if len(tiff) < 8 {
		return nil, 0, false
	}
	var bo binary.ByteOrder
	switch string(tiff[0:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil, 0, false
	}
	if bo.Uint16(tiff[2:4]) != 42 {
		return nil, 0, false
	}
	off := int(bo.Uint32(tiff[4:8]))
	if off < 8 || off > len(tiff) {
		return nil, 0, false
	}
	return bo, off, true
}

// readIFD 读出 off 处的 IFD 条目；越界或类型未知时返回 false
func readIFD(tiff []byte, bo binary.ByteOrder, off int) ([]exifEntry, bool) {
	if off < 0 || off+2 > len(tiff) {
		return nil, false
	}
	count := int(bo.Uint16(tiff[off : off+2]))
	base := off + 2
	if base+count*12+4 > len(tiff) {
		return nil, false
	}

	entries := make([]exifEntry, 0, count)
	for i := 0; i < count; i++ {
		raw := tiff[base+i*12 : base+i*12+12]
		e := exifEntry{
			tag:   bo.Uint16(raw[0:2]),
			typ:   bo.Uint16(raw[2:4]),
			count: bo.Uint32(raw[4:8]),
		}
		unit := exifTypeSize(e.typ)
		if unit == 0 || e.count == 0 {
			continue // 类型不认识或空值：跳过这一个标签
		}
		total := unit * int(e.count)
		switch {
		case total < 0 || total > len(tiff):
			return nil, false // 长度离谱，判定为结构异常
		case total <= 4:
			e.value = bytes.Clone(raw[8 : 8+total])
		default:
			dataOff := int(bo.Uint32(raw[8:12]))
			if dataOff < 0 || dataOff+total > len(tiff) {
				return nil, false
			}
			e.value = bytes.Clone(tiff[dataOff : dataOff+total])
		}
		entries = append(entries, e)
	}
	return entries, true
}

func exifTypeSize(typ uint16) int {
	switch typ {
	case 1, 2, 6, 7: // BYTE / ASCII / SBYTE / UNDEFINED
		return 1
	case 3, 8: // SHORT / SSHORT
		return 2
	case 4, 9, 11: // LONG / SLONG / FLOAT
		return 4
	case 5, 10, 12: // RATIONAL / SRATIONAL / DOUBLE
		return 8
	}
	return 0
}

func entryUint32(bo binary.ByteOrder, e exifEntry) uint32 {
	switch {
	case len(e.value) >= 4:
		return bo.Uint32(e.value[:4])
	case len(e.value) == 2:
		return uint32(bo.Uint16(e.value))
	}
	return 0
}

// buildTIFF 重新拼装：TIFF 头 + IFD0 + Exif IFD + 外部数据区。
// 所有偏移都相对 TIFF 头起点，删掉标签后必须重算。
func buildTIFF(bo binary.ByteOrder, ifd0, exif []exifEntry) []byte {
	sort.Slice(ifd0, func(i, j int) bool { return ifd0[i].tag < ifd0[j].tag })
	sort.Slice(exif, func(i, j int) bool { return exif[i].tag < exif[j].tag })

	if len(exif) > 0 {
		// 先占位，偏移在上面排序后再回填
		ifd0 = append(ifd0, exifEntry{tag: exifTagExifPointer, typ: 4, count: 1, value: make([]byte, 4)})
		sort.Slice(ifd0, func(i, j int) bool { return ifd0[i].tag < ifd0[j].tag })
	}

	ifd0Size := 2 + 12*len(ifd0) + 4
	exifSize := 0
	if len(exif) > 0 {
		exifSize = 2 + 12*len(exif) + 4
		for i := range ifd0 {
			if ifd0[i].tag == exifTagExifPointer {
				bo.PutUint32(ifd0[i].value, uint32(8+ifd0Size))
			}
		}
	}
	// 外部数据区排在两个 IFD 之后
	dataOff := 8 + ifd0Size + exifSize

	out := make([]byte, 8, dataOff)
	if bo == binary.LittleEndian {
		copy(out, "II")
	} else {
		copy(out, "MM")
	}
	bo.PutUint16(out[2:4], 42)
	bo.PutUint32(out[4:8], 8)

	var data []byte
	writeIFD := func(entries []exifEntry) {
		head := make([]byte, 2)
		bo.PutUint16(head, uint16(len(entries)))
		out = append(out, head...)
		for _, e := range entries {
			entry := make([]byte, 12)
			bo.PutUint16(entry[0:2], e.tag)
			bo.PutUint16(entry[2:4], e.typ)
			bo.PutUint32(entry[4:8], e.count)
			if len(e.value) <= 4 {
				copy(entry[8:12], e.value)
			} else {
				bo.PutUint32(entry[8:12], uint32(dataOff+len(data)))
				data = append(data, e.value...)
			}
			out = append(out, entry...)
		}
		out = append(out, 0, 0, 0, 0) // 没有下一个 IFD
	}
	writeIFD(ifd0)
	if len(exif) > 0 {
		writeIFD(exif)
	}
	return append(out, data...)
}
