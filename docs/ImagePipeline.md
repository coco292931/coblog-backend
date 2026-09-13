# 图片管线：现状与优化方案

> 图例：🔲 = 待做（未标注的条目是现状描述，不是待办）
> 涉及仓库：`coblog-backend`（上传 / 压缩 / 静态服务）、`coblog-frontend`（展示 / 下载）
> 相关代码：`services/fileService/fileStorageSvc.go`、`controllers/fileController/{upload,serveUpload}.go`、`configs/router/router.go`、前端 `src/utils/image.js` 与 `pages/article/index.vue`
> 难度与工作量估算见「三、实现难度评估」；本次评审的改动见文末「六、修订记录」

---

## 一、现状

> ⚠️ 本节记录的是 P0-a 实施**之前**的行为；已变更的部分见「六、实施记录」。

### 1. 上传与压缩（`SaveImageWithCompression`）

| 环节 | 现状 |
|---|---|
| 上传上限 | `UploadImage` 限 10,240,000 字节（注释写 10 MiB，实际约 9.8 MiB），超限直接拒 |
| 扩展名白名单 | `.jpg` `.jpeg` `.png` `.gif` `.webp`，**取自文件名**（不是内容嗅探） |
| 是否生成压缩图 | `compress_threshold == 0`（总是压）**或** `len(data) > threshold`；当前 `524288` → **严格大于 512 KiB** 才有压缩图 |
| 原图 | **始终逐字节落盘**，不受阈值影响 |
| 解码 | `.png` → `png.Decode`；其余 → `imaging.Decode`（标准库 `image.Decode`） |
| 缩放 | 仅当**宽度** > `compress_max_width`（当前 1920）时 `imaging.Resize(..., 0, Lanczos)`，**不限高度** |
| 编码 | `.png` → `png.Encode`（默认压缩级别，不看有无 alpha）→ `_c.png`；其余 → `jpeg.Encode`（`compress_quality`，当前 80，越界回退 80）→ `_c.jpg` |
| 失败降级 | 解码/编码/写盘任一失败 → 记 WARN、**只保留原图**，上传仍然成功（响应 `compressed: false`） |
| 方向信息 | ⚠️ **压缩图丢失 EXIF Orientation**：`imaging.Decode` 默认不自动旋转，重编码后不含方向信息 → 竖拍手机照片的 `_c.*` 会躺倒（原图靠浏览器自动旋转，是正的） |
| 解码前校验 | ⚠️ **无**：`io.ReadAll` 全量入内存 + 全量 `Decode`，不校验像素数（见问题 #14） |

### 2. 命名

- 原图 `<base><原后缀>`，压缩图 `<同一个 base>_c<压缩后缀>`
- `base = randStrGenerater(32)`，字符集仅 `A-Z0-9`（无 `_` `.`，所以 `<base>_c.` 前缀匹配不会串图）

### 3. 读取（`ServeUpload`，接管原 `gin.Static`）

- 路由：`GET/HEAD /static/uploads/*filepath`
- `?thumb=1` → 在目录里找 `<base>_c.*`（前缀匹配，多个候选**优先 `.png`**），**找不到则回退原图**
- 响应头 `X-Image-Variant: thumb|original` 告知实际返回哪种（已加入 CORS `ExposeHeaders`）
- 路径收敛为纯文件名，含 `..` / `%2e` / `/` 一律 404；`gin.Dir(dir,false)` 禁目录列举
- 内部 `http.StripPrefix` + `http.FileServer` → Range / 304 / Content-Type 等原生行为保留

### 4. 前端消费

- 正文 / 封面只存**原图地址**；展示时拼 `?thumb=1`（`src/utils/image.js` 的 `thumbUrl` / `stripThumb`）
- 正文 HTML 在交给 `v-html` **之前**先把 `<img src>` 改写成 `?thumb=1`，避免浏览器解析时先拉原图
- 缩略图先渲染，原图后台加载完成后替换（不依赖点开/放大）
- 灯箱「原图/压缩图」按钮由一次 `HEAD` 读 `X-Image-Variant` 决定；下载走原图（`fetch` 带 `cache:'no-cache'`）
- ⚠️ 该 `HEAD` 探测**无缓存**，每次打开灯箱都会重新发一次（见问题 #19）；`thumbUrl` 只对含 `/static/uploads/` 的地址加参数、外链原样返回（这部分已正确）

### 5. 线上现状

生产 API 前面有 **Cloudflare**，`/static/uploads/*` 当前响应：

```
Cache-Control: max-age=43200     ← 由 CF 注入，Go 侧没有设 Cache-Control，也没有 ETag
Last-Modified: ...               ← 有
cf-cache-status: REVALIDATED     ← 边缘有缓存，但 max-age 到期就回源协商
Accept-Ranges: bytes
```

---

## 二、问题清单（按影响排序）

| # | 问题 | 影响 |
|---|---|---|
| 1 | 缓存策略由 CF 决定（12 小时），文件名随机且内容永不改变却仍在反复协商 | 每次冷启动/过期都多一轮 304 请求；CF 命中率与源站压力都被浪费 |
| 2 | PNG 一律重编码为 PNG | 压完可能**比原图更大**（已实测：某图只小 2%）；无 alpha 的截图白白保持大体积 |
| 3 | 只限宽不限高 | 长截图 / 全景图完全不缩，压缩图可能仍有数 MB |
| 4 | `.webp` 允许上传但**无解码器**（`imaging` 未注册 webp 解码器） | 静默不压缩（`compressed` 恒 false），是"允许上传但功能不可用"的最差状态 |
| 5 | **原图保留 EXIF/GPS** | 手机照片的定位信息会随「下载原图」公开 |
| 6 | GIF / 带 alpha 的 webp 压成 JPEG | 丢失动画（只取第一帧）、透明区变黑 |
| 7 | 没有图片元信息表（宽高/字节/哈希） | 前端无法用 `<img width height>` 占位（布局跳动）；无法统计压缩率、无法批量重压 |
| 8 | 没有按内容去重 | 同一张图重复上传 / 多篇文章引用会存多份 |
| 9 | 没有清理机制 | 孤儿文件（文章已删、上传失败）永久累积；也没有依据判断"哪些文件还被引用" |
| 10 | 图片目录没有备份 | DB 有备份、文件没有，是唯一的单点丢失风险 |
| 11 | 无格式/尺寸协商 | 移动端拿到的仍是给桌面准备的 1920 宽图；没有 srcset |
| 12 | 配置项语义易踩 | `compress_threshold: 0` 表示"总是压缩"，容易被误当成"不压缩"；比较是严格 `>` |
| 13 | **压缩图丢失 EXIF 方向**（`imaging.Decode` 不自动旋转，`_c.*` 无 Orientation） | 竖拍手机照片：原图正、缩略图躺倒。**现存 bug，一行可修** |
| 14 | 解码前不校验像素数 | 「解压炸弹」：10 MiB 的 PNG 可解成数亿像素（数 GB 内存）→ OOM / DoS |
| 15 | 后缀取自文件名，不做内容嗅探 | 改名的 jpg 当 `.png` 传 → 解码失败、静默不压缩；白名单形同虚设 |
| 16 | `thumbName` 每次请求都 `os.ReadDir` 整个目录 | 单目录上万文件后，每个 `?thumb=1` 都是 O(n) 目录扫描；与「一个 base 多个变体」的方向直接冲突 |
| 17 | 去重与「孤儿清理」互相矛盾 | 去重后同一文件被 N 篇文章引用，「无引用即可删」失效；要么引用计数，要么明确永不删 |
| 18 | RSS 的 `media:thumbnail` 用原图 URL，`type` 由 URL 后缀推断（`rssService/mapper.go`） | 阅读器拉缩略图会拉整张原图；P0-a-2 把无 alpha 的 PNG 压成 JPG 后 `type` 与实际内容不符 |
| 19 | 灯箱每次打开都发一次 `HEAD` 探测，无缓存 | 同一图片重复请求；上传响应里的 `compressed` 本可覆盖常见场景 |

> 排序说明：#13 是**线上现存 bug**（一行可修），实际优先级与 #1 同级。
> #4 的依赖成本已修正：`golang.org/x/image` 本就是间接依赖（2019 版），vendor 里没有 `webp` 包只是因为 vendor 只收被 import 的包。
> ⏸ **本次范围**：只做 P0-a + P0-b（**仅对新增上传生效，不重处理历史图**）→ 依赖 `images` 表的 #7 / #9 / #16 与内容寻址 #8 一并暂缓。

---

## 三、实现难度评估

| 条目 | 工作量 | 风险 | 卡点 / 依赖 |
|---|---|---|---|
| P0-a-1 长缓存 | 0.5h | 低 | **一半在 CF 控制台**，不是代码；与「重压老图」冲突 |
| P0-a-2 压缩规则修正 | 3~5h | 中低 | alpha 判断时机（见下）；要配单测 |
| P0-a-3 `.webp` 决策 | 1~2h | 低 | 比初稿更简单：依赖已在，只需 `go mod vendor` |
| P0-a-4 方向修复 | **0.5h** | 极低 | 一行 + 一张带 Orientation 的单测图 |
| P0-a-5 像素上限 | 2~3h | 低 | 需要定义错误码 / 提示文案 |
| P0-a-6 内容嗅探 | 1~2h | 低 | `mimetype` 已是间接依赖 |
| P0-b-1 元数据剥离 | **8~12h** | 中高 | 已定「字节可变、像素不变」→ 要重建 EXIF（白名单 + 重算偏移）；异常时整段丢 APP1 |
| ⏸ P1-7 `images` 表 | 4~8h | 中 | 本次暂缓（涉及历史图重处理） |
| ⏸ P1-8 内容寻址 | 2~3h | 中 | 本次暂缓，与 P1-7 一起做 |
| ⏸ P1-9 按内容去重 | 2h + 设计 | 中高 | 本次暂缓；动手前先定 #17 |
| P1-10 格式协商 | **1~2 天** | **高** | webp **编码器**：`CGO_ENABLED=0` → `scratch`，cgo 方案出局 |
| P1-11 多档 + srcset | 2~3 天 | 中 | lookup 逻辑要重写（前缀匹配 → 表驱动）+ 前端布局 |
| P2 | 数周 | — | — |

**P0-a（1~6）合计约半天**，且全部落在 `fileStorageSvc.go` / `serveUpload.go` 内。
`services/fileService` **不依赖 `configs/database`**，可直接 `go test ./services/fileService/`
（不需要 `go test -c` 那套变通，那只对 `fileController` 必要）。

---

## 四、方案

### P0-a — 纯收益（半天） ✅ 已实现（2026-09-13，落地情况见「六、实施记录」）

**1. 显式长缓存** 🔲

```go
// ServeUpload 里，委托 FileServer 之前
c.Header("Cache-Control", "public, max-age=31536000, immutable")
```

- 位置可行：`http.FileServer` 只补 `Last-Modified`，**不会覆盖** `Cache-Control`；HEAD / Range / 304 都会带上这个头（正是想要的）。
- CF 侧二选一：给 `/static/uploads/*` 配长 TTL 的 **Cache Rule**（推荐，作用域最小），或把 Zone 级 Browser Cache TTL 设为「Respect Existing Headers」。
- 建议**同时给 ETag**（或直接靠内容寻址的文件名）：现在只有 `Last-Modified`，CF 的 12h 到期后仍会回源做 `If-Modified-Since` 协商。

> ⚠️ 前提：**文件名永不复用**。
> ✅ 已决策「不做历史重处理」→ **immutable 现在可以独立上线**（改压缩参数后新图是新随机名，老图内容也不变）。
> 将来若真要重压老图，必须先做 P1-8 的内容寻址，否则边缘/浏览器会把旧内容钉住一年。

**2. 压缩规则修正** 🔲

- **alpha 感知（依据「解码后有无真实透明像素」，不再看源后缀）**：有透明像素 → `png.Encode` → `_c.png`；无 → `jpeg.Encode` → `_c.jpg`
  ⚠️ **不能只看 `ext == ".png"`**：启用 webp 解码后，带 alpha 的 webp（以及 GIF 透明区）如果一律走 JPEG，**透明区会变黑**——这是现在「不压缩」掩盖住的问题，启用解码后会暴露。
  ⚠️ **必须在 `Resize` 之前判断**：`imaging.Resize` 返回 `*image.NRGBA`（灰度图为 `Gray`），而 `NRGBA` 正在下面的白名单里 → resize 之后再判断会**永远返回 true**，所有 PNG 依旧走 PNG 编码。单测要专门钉住这个坑。
  ```go
  // 只有可能带 alpha 的颜色模型才需要逐像素确认
  func hasAlpha(img image.Image) bool {
      switch img.(type) {
      case *image.NRGBA, *image.RGBA, *image.NRGBA64, *image.RGBA64, *image.Paletted:
      default:
          return false
      }
      b := img.Bounds()
      for y := b.Min.Y; y < b.Max.Y; y++ {
          for x := b.Min.X; x < b.Max.X; x++ {
              if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
                  return true
              }
          }
      }
      return false
  }
  ```
- **压完不划算就别存**：改用**比例阈值**（如 `len(compressed) >= len(data)*95/100` 就丢弃），而不是严格 `>=`——只小 2% 的图留着，只会让「变体是否存在」更难预测。
- **加高度上限**：配置新增 `compress_max_height`（如 3840），`Resize` 按"限制长边"处理；⚠️ 新增配置项要同步 `appConfigs_example.yaml`（只有它被 git 跟踪）。
- **JPEG 质量：已可配置，不需要新代码**（`compress_quality`，越界/为 0 时回退 80）
  - ✅ **决策：缩略图以网络与响应速度为优先 → 下调到 75**；两个地方都要改：`appConfigs_example.yaml` + 本机未被 git 跟踪的 `appConfigs.yaml`（不改本机的不生效）。
  - 依据：Go 标准库编码器**恒定使用 4:2:0 色度子采样**（`image/jpeg` writer 里写死，不随 quality 变），所以 **quality 是唯一的体积旋钮**，而 `jpeg.DefaultQuality` 就是 75。80 → 75 通常省 10%~15% 字节，肉眼几乎无差；若想再激进可试 70（缩略图场景可接受）。
  - 顺手把代码里的回退值 80 改成 `jpeg.DefaultQuality`（75），避免配置缺失时又回到 80。

**3. `.webp`：引入解码器（已决策）** 🔲

- ✅ **决策：引入解码器**。`golang.org/x/image` **已是间接依赖**（`go.mod` 里 v0.0.0-20191009），vendor 里没有 `webp` 包只是因为 vendor 只收被 import 的包 → 加一行 `_ "golang.org/x/image/webp"` + `go mod vendor` 即可，**不需要新引模块**。
- ✅ **动图 webp：取首帧**（与 GIF 现状一致——`image/gif` 的 `Decode` 也只解第一帧）。
  - ⚠️ 该版本解码器**遇 ANIM 段会直接报错**，所以「取首帧」= 自己走一遍 RIFF chunk：定位 `VP8X` → 判 ANIM 标志 → 取第一个 `ANMF`，剥掉 16 字节帧头，把其中的 `VP8`/`VP8L` 子块**重新包成最小 RIFF 容器**再交给 `webp.Decode`（约 80~120 行 + 单测）。
  - 备选（不想写这段时）：动图 webp **明确拒收**并报错，代码为零。
  - ⚠️ 若顺手升到新版 `x/image`，vendor 里的 bmp/tiff 也会跟着变，需要重跑一遍图片单测。

**4. 修方向（新增）** 🔲

压缩图丢方向的 bug（#13）一行可修：

```go
// decodeImage：让解码器按 EXIF Orientation 把像素转正
return imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
```

`imaging` ≥ 1.6.0 支持（当前 1.6.2）。**与 P0-b 的方向取舍联动**：压缩图转正后，原图仍带 EXIF、
浏览器会自动旋转 → 两者最终朝向一致，这正是期望行为。

**5. 解码前卡像素上限（新增）** 🔲

`image.DecodeConfig` 先读头部，超过上限（建议新增 `max_pixels`，如 1 亿像素）直接按「文件不可处理」拒绝，
不做全量 `Decode`。顺带白拿宽高，P1-7 要用（不必再解一次）。

**6. 内容嗅探（新增，可选）** 🔲

用 `github.com/gabriel-vasile/mimetype`（**已是间接依赖**）按内容判定类型，与白名单交叉校验；
至少要做到「后缀与内容不符时明确报错」，而不是静默不压缩。

**验收（P0-a）** ✅
- [x] `curl -I` 静态图片能看到 `Cache-Control: ... immutable`（handler 单测断言）
- [x] 无 alpha 的 PNG 压出 `_c.jpg`；带 alpha 的仍为 `_c.png`；带 alpha 的 webp / GIF 也走 `_c.png`、透明区不变黑（单测覆盖）
- [x] **单测钉住「resize 之后再判断 alpha 会误判」这个坑**
- [x] 压缩收益不足 5% 的图不再产生 `_c.*`（单测覆盖）
- [x] 超高大图被限制到 `compress_max_height` 以内
- [x] `.webp` 能正常压缩；动图 webp 压出静态首帧（真实样本单测覆盖），不再是静默降级
- [x] 带 EXIF Orientation=6/8 的照片，`_c.*` 已按方向转正（单测覆盖）
- [x] 超大像素的图片被明确拒绝，不会 OOM
- [x] 压缩图 JPEG 质量按 `compress_quality: 75` 生效

### P0-b — 原图剥离位置信息（决策已定） 🔲

**契约（已定）**：原图**允许改写字节，但像素一字不动** → 只能做容器级过滤，**不能重编码、不能烘焙旋转**。

**保留白名单（EXIF 标签）**

| 类别 | 标签 |
|---|---|
| 作者 / 版权 | `Artist(0x013B)`、`Copyright(0x8298)` |
| 设备 | `Make(0x010F)`、`Model(0x0110)`、`LensMake(0xA433)`、`LensModel(0xA434)`、`LensSpecification`、`BodySerialNumber`、`LensSerialNumber` |
| 拍摄参数 | `ExposureTime(0x829A)`、`FNumber(0x829D)`、`ISOSpeedRatings(0x8827)`、`FocalLength(0x920A)`、`FocalLengthIn35mmFilm(0xA405)`、`ExposureProgram`、`ExposureBiasValue`、`MeteringMode`、`Flash`、`WhiteBalance`、`SceneCaptureType` |
| 时间 | `DateTime(0x0132)`、`DateTimeOriginal(0x9003)`、`DateTimeDigitized(0x9004)`、`OffsetTime*` |
| 结构必需 | `Orientation(0x0112)`、`ExifOffset(0x8769)`、`XResolution` / `YResolution` / `ResolutionUnit`、`ColorSpace`、`PixelXDimension` / `PixelYDimension`、`ComponentsConfiguration` |

**剥离**：`GPSInfo(0x8825)` 及其整个 GPS IFD、`MakerNote`（体积大且无保留价值）、`UserComment`、`ImageDescription`（可能写地点）、`Software`。

**同时处理**：`APP2 (ICC)` **必须保留**（否则广色域照片褪色）；`APP1` 里的 XMP、`APP13 (IPTC)`、`COM` **一律丢弃**（都可能写位置）。

**实现路线：重建 EXIF 段**（不是「丢整段 APPn」）

- 解析 TIFF header → IFD0 / Exif IFD → 按白名单筛选 → **重算偏移** → 重新拼装，约 200~250 行 + 单测。
- ⚠️ 偏移量都相对 TIFF header 起点，删标签后必须重算；IFD1（内嵌缩略图）**建议整体丢弃**（无保留价值）。
- 不引外部库（`rwcarlsen/goexif` 只读、`dsoprea/go-exif` 太重），手写约 200 行可控。
- ⚠️ **`Orientation` 必须留下**：像素不动，就只能靠标签让浏览器转正。与 P0-a-4（缩略图烘焙转正）配合，两者最终朝向一致。
- ⚠️ **只对新增上传生效**（已决策不做历史重处理）→ 老图仍带 GPS；日后若要清理，得先补 P1-7 的批量能力。
- PNG：无方向语义（浏览器不会按 EXIF 旋转 PNG，丢了不会躺倒），`eXIf` / `tEXt` / `iTXt` / `zTXt` **直接丢弃**，不做白名单重建。

**验收（P0-b）**
- [ ] 原图不含任何 `GPS*` 标签（`exiftool` 确认）
- [ ] 作者 / 版权 / 机型 / 镜头 / 曝光 / 光圈 / 快门 / ISO / 焦距 / 拍摄时间仍在（`exiftool` 逐项确认）
- [ ] ICC 配置保留（广色域照片颜色不变）
- [ ] 竖拍照片原图朝向正确，且**解码后像素哈希与上传前一致**（证明没动像素）
- [ ] 元数据异常的图：APP1 被整段丢弃，**不产生半损坏文件**（图片仍能正常解码）
- [ ] XMP / IPTC / COM 已不存在于输出（`exiftool` / `strings` 抽查）

**已决策的边界情况**
1. ✅ EXIF 解析失败 / 结构异常 → **整段丢弃 APP1**（宁可丢参数、绝不泄漏位置）
2. ✅ XMP（APP1）与 IPTC（APP13）→ **一并丢弃**（作者 / 版权已由 EXIF 的 `Artist` / `Copyright` 保留）
3. ✅ PNG 的 `eXIf` 段 → **直接丢弃**（PNG 无方向语义，不会躺倒；代价是丢截图工具写入的参数）

### P1 — 结构性改造（1~3 天）｜⏸ 本次暂缓，先记录方案

**7. 建 `images` 表（地基）** ⏸

```
images(id, base, orig_ext, orig_bytes, orig_w, orig_h, sha256,
       variants_json, created_at, ref_count, last_ref_at)
```

解锁：去重、**变体查找**（替掉 `os.ReadDir`）、宽高元信息、压缩率观测、老图批量重压、孤儿判定。

- **必须由它承担「base → 变体文件名」的映射**：现在 `thumbName` 每次请求都 `os.ReadDir` 整个目录（#16），
  单目录上万文件后是 O(n)；P1-8/P1-11 会让「一个 base 多个变体」成为常态，前缀匹配式 lookup 撑不住。
  `os.ReadDir` 只保留为**老文件兼容路径**（新表查不到时回退）。
  （⏸ 暂缓的直接后果：#16 的 O(n) 目录扫描会一直保留；当前规模可接受）
- `autoMigrate` 已开启 → 新表安全；但模型标签要与 `configs/database` 的既有风格一致（参考仓库笔记里的 AutoMigrate 陷阱）。
- 顺手加一个**扫描现有目录回填**的启动任务（参考 `configs/database/database.go` 的 `backfillActivation` 先例）。

**8. 内容寻址 + 版本化 URL** 🔲 ⏸ 与 P1-7 合并，暂缓

`base` 由随机串改为内容哈希（如 `sha256[:16]`）；变体可再带生成参数（`?thumb=1&v=<配方哈希>`），使"换质量/换格式重压"时 URL 自然变化，immutable 缓存依然安全。
⚠️ 保留老随机文件名的读取兼容。**P0-a-1 的 immutable 已不需要再等它**（已决定不做历史重处理）。

**9. 按内容去重** 🔲 ⏸ 暂缓

`sha256` 命中已有记录直接复用 URL，不再写第二份。

⚠️ **与「孤儿清理」互斥**（#17）：去重后一个文件可能被 N 篇文章引用，「无引用即可删」不再成立。
二选一（**先定下来再动手**）：
1. **明确永不删文件**（简单、安全；配合廉价存储，P2 上对象存储后成本更低）；
2. 在 `images` 表里维护引用计数，但代价是需要定期**全量扫描文章正文**统计引用
   （`LIKE '%"base"%'` 用不上索引，成本随文章量增长）。

**10. 格式协商（`?thumb=1` 设计的红利）** 🔲 ⚠️ 建议降级 / 推迟

按 `Accept: image/avif,image/webp` 返回最省格式，**正文地址一个字都不用改**。

⚠️ 真正的难点是**编码器**：`Dockerfile` 是 `CGO_ENABLED=0` → `scratch`（无 libc），
`github.com/chai2010/webp` 这类 cgo 方案**直接出局**；纯 Go 选项（如 `HugoSmits86/nativewebp`，仅无损 VP8L）收益有限。
建议：只做 P0-a-3 的「上传的 webp 能解码/转码」，格式协商**推到 P2** 与对象存储 / CDN 一起做。

**11. 多档尺寸 + srcset** 🔲

`?thumb=1&w=480|960|1920`，前端配 `srcset/sizes`；按需生成 + 落地缓存。
⚠️ 命名要升级（如 `<base>_c_<w>.<ext>`），`thumbName` 的"前缀匹配 + 优先级"逻辑需要**重写**，
并与 P1-7 的表驱动查找合流。建议等有真实移动端流量数据再做。

**12. RSS 端点同步（新增）** 🔲

- `media:thumbnail` 改为 `?thumb=1`（现在给的是原图 URL，阅读器会拉整张原图）
- `enclosure type` 不再由 URL 后缀推断：P0-a-2 之后无 alpha 的 PNG 会变成 JPG，
  `type="image/png"` 会与实际内容不符（`rssService/mapper.go` 的 `ImageMIME(coverURL)`）

### P2 — 规模上来再说

13. 对象存储 + CDN（`ServeUpload` 改为签名或 302；与现有 CF 天然合拍）
14. AVIF、渐进式 JPEG、LQIP 模糊占位（占位图存 `images` 表）
15. 异步变体生成队列（当前同步做足够）
16. 观测：压缩率分布 / 目录占用 / 无压缩图比例 / CF 命中率
17. 前端：灯箱 `HEAD` 探测结果加内存缓存（#19）；封面 / 正文改用 `<img width height>` 防布局跳动（依赖 P1-7 的元信息）

### 不做

- 预生成「尺寸 × 格式」矩阵 → 按需生成 + 落地缓存即可
- 引入 imgproxy / Thumbor 之类完整图片服务 → 除非要 AVIF / 动态裁剪
- 分布式存储、多副本 → 先把备份做扎实

---

## 五、落地顺序与注意事项

**建议顺序**

1. **P0-a（半天）**：`1 + 2 + 3 + 4 + 5 + 6`。全部集中在两个文件里，无外部依赖，先拿收益（含 #13 的现存 bug）。
2. **P0-b（1 天）**：原图剥离位置信息，保留相机 / 作者 / 版权元数据（决策已全部拍板）。
3. **P1-12 RSS 同步**：不依赖任何改造，可与 P0-a 一起上。
4. ⏸ **P1-7 / 8 / 9 / 10 / 11 全部暂缓**：都涉及历史图重处理或需要 `images` 表，本次不做。

**注意事项**

1. **后端先行**：前端依赖 `?thumb=1` 与 `X-Image-Variant`，老后端会忽略参数（展示会退回原图，更重但不坏）。
2. **契约（已更新）**：数据库里存的始终是**原图 URL**（URL 不变）；P0-b 之后原图**字节会变、像素不变** → 压缩/格式/尺寸仍是"服务端在同一个 URL 后面做决策"，各条可独立上线。
3. **immutable 已无冲突**：既然不做历史重处理，P0-a-1 可以直接上线；只有将来真要重压老图时，才必须先补 P1-8 的内容寻址。
4. **cf-cache-status 会骗人**：CF 命中时源站日志看不到请求；排查"到底走了缓存吗"要看响应头而不是后端日志。
5. **本地验证环境**：`fileobject.dir` 用真实存在的目录即可（`ServeUpload` 现在会在 `?thumb=1` 时 `os.ReadDir`）。
6. **测试落点**：压缩 / 命名逻辑都在 `services/fileService`，该包**不依赖 `configs/database`** → 可直接 `go test ./services/fileService/`；
   只有 `controllers/*` 才需要 `go test -c` + 在仓库根目录执行测试二进制那套变通（`configs/database` 有包级 `init()`）。
7. **单测素材**：需要一张 `Orientation=6/8` 的 JPEG、一张带 alpha 的 PNG、一张带 GPS 的 JPEG、一张大尺寸长截图（生成脚本或小体积样本，注意仓库体积）。
8. **不做历史重处理**：老图仍带 GPS、仍是旧压缩参数；只有新上传的图干净。要做好「新旧共存」的心理准备，别指望线上图库整体变干净。

---

## 六、实施记录

### P0-a（2026-09-13 完成）

| 项 | 落地情况 |
|---|---|
| 1 长缓存 | `ServeUpload` 加 `Cache-Control: public, max-age=31536000, immutable` 与 `ETag`（取实际发回的文件名，可换 304）；404 不带长缓存（`http.serveFile` 会清掉这几个头） |
| 2 压缩规则 | alpha 按**解码后**有无透明像素判断（必须在缩放前，代码里已注明原因）；体积收盏 ≥5% 才留压缩图；`compress_max_height`（3840）与 `compress_max_width` 同时生效；两处配置的 `compress_quality` 都降到 75，代码回退值改为 `jpeg.DefaultQuality` |
| 3 `.webp` | 引入 `golang.org/x/image/webp`（`go mod tidy` 后为直接依赖）；动图按 RIFF/ANMF 剥首帧，带 ALPH 时补一个只声明 alpha 的 VP8X |
| 4 方向 | `imaging.Decode(..., imaging.AutoOrientation(true))` |
| 5 像素上限 | 落盘前用 `image.DecodeConfig` 卡 `max_pixels: 100000000`，超限返回新错误码 **4007** |
| 6 内容嗅探 | 实现改为**魔数自查**（`imageTypeByContent`），未引入 `mimetype`：`vendor/` 不入库，能少一个依赖就少一个 |

**与原方案的差异**

- 嗅探结果**直接作为图片类型**（而不是「后缀与内容不符就报错」）：改名的图也能正常压，更宽容。
- `readFileData` 不再返回后缀，`SaveImageWithCompression(data)` 只收字节。
- 新增错误码 `4007 图片像素过多，无法处理`；`upload.go` 会把这类可预期错误原样透传（不再一律当成 `4006`）。

**测试**

- `services/fileService/fileStorageSvc_test.go`（自造素材，不依赖外部文件）：类型嗅探 / alpha 判定（含「缩放后判断会误判」的钉子）/ 宽高上限 / 收益阈值 / 像素上限 / EXIF 方向 / 动图首帧容器（含带 ALPH 的组合）
- 动图与带 alpha 的 webp 另在本地用 ffmpeg 样本验证过（静态、静态带 alpha、动图、动图带 alpha 四份），样本不入库
- `controllers/fileController/serveUpload_test.go`：补缓存头与 304 断言
- 跑法：`go test ./services/fileService/`；`fileController` 仍需 `go test -c` + 仓库根目录执行

---

## 七、修订记录

**2026-09-13（待确认项全部拍板）**

- ✅ EXIF 解析失败 / 结构异常 → **整段丢弃 APP1**（安全优先）。
- ✅ XMP（APP1）、IPTC（APP13）、`COM` → **一并丢弃**。
- ✅ PNG 的 `eXIf` / `tEXt` / `iTXt` / `zTXt` → **直接丢弃**，不做白名单重建。
- ✅ `.webp` → **引入解码器**（`golang.org/x/image/webp`，一行 import + `go mod vendor`）；**动图取首帧**（需自己剥 RIFF/ANMF 容器，备选是零代码的「明确拒收」）。
- 顺带修正 P0-a-2：alpha 判断依据从「源后缀是 .png」改为「**解码后有无真实透明像素**」——启用 webp 解码后，带 alpha 的 webp / GIF 若走 JPEG 会**透明变黑**（现在「不压缩」掩盖了这个问题）。
- 至此 P0-a / P0-b 无未决项，可直接开工。

**2026-09-13（决策确认）**

- ✅ **不做历史图片重处理**，管线只对新增上传生效 → P1-7 `images` 表、P1-8 内容寻址（与之合并）、P1-9 去重、P1-10 / P1-11 全部暂缓；P0-a-1 的 immutable 因此可独立上线。
- ✅ **原图允许改写字节，但像素一字不动** → P0-b 走「重建 EXIF 段」路线（保留白名单元数据 + 剥离 GPS），不做重编码 / 烘焙旋转；`Orientation` 必须保留。
- ✅ **缩略图优先网络与响应速度** → `compress_quality` 由 80 下调到 **75**（Go 编码器恒定 4:2:0，quality 是唯一体积旋钮）。
- P0-b 补「保留 / 剥离」逐项清单（作者、版权、机型、镜头、曝光/光圈/快门/ISO/焦距、拍摄时间）、约 200~250 行实现估量与 3 个待确认项。

**2026-09-13（据代码逐条核对）**

- 现状表补「方向信息」「解码前校验」两行；前端消费补 `HEAD` 探测无缓存。
- 问题清单补 #13~#19（#13 为现存线上 bug）；修正 #4 的依赖成本描述。
- 新增「三、实现难度评估」（工作量 / 风险 / 卡点）。
- 方案拆为 **P0-a（纯收益，半天）** 与 **P0-b（需先决策）**；原 P0-4 移入 P0-b。
- 修正 alpha 判断时机（必须在 `Resize` 之前）、「不划算」改用比例阈值、webp 依赖成本、`Cache-Control` 的 ETag / Cache Rule 细节。
- P1 重排：`images` 表提到最前并承担变体查找；补去重与清理的矛盾、RSS 同步（P1-12）；格式协商标注 cgo 限制并建议推迟。
