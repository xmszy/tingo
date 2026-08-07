// Package image 提供图片处理工具函数。
//
// 基于 Go 标准库 image 包，零外部依赖。支持：
//   - 缩放（Resize）：等比/拉伸/按宽/按高 缩放，支持近邻和双线性插值
//   - 裁剪（Crop）：从中心或指定锚点裁剪
//   - 缩略图（Thumbnail）：等比缩放后居中裁剪到指定尺寸
//   - 水印（Watermark）：图片水印叠加（可调透明度和比例）
//   - 格式转换：JPEG/PNG/GIF 互转，可控质量
//
// 用法：
//
//	img, format, _ := image.Decode(reader)
//	resized := timage.Resize(img, 800, 600, ResizeFit)
//	jpeg.Encode(writer, resized, &jpeg.Options{Quality: 85})
package image

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// ResizeMode 缩放模式。
type ResizeMode int

const (
	// ResizeFit 等比缩放，完整容纳在目标区域（可能留白）。
	ResizeFit ResizeMode = iota
	// ResizeFill 等比缩放，填满目标区域（可能裁切）。
	ResizeFill
	// ResizeStretch 拉伸缩放，忽略宽高比。
	ResizeStretch
	// ResizeWidth 按宽度等比缩放。
	ResizeWidth
	// ResizeHeight 按高度等比缩放。
	ResizeHeight
)

// Anchor 裁剪/水印锚点。
type Anchor int

const (
	AnchorCenter      Anchor = iota // 居中
	AnchorTopLeft                   // 左上
	AnchorTopRight                  // 右上
	AnchorBottomLeft                // 左下
	AnchorBottomRight               // 右下
	AnchorTop                       // 上中
	AnchorBottom                    // 下中
	AnchorLeft                      // 左中
	AnchorRight                     // 右中
)

// Resize 缩放图片到指定宽高。
// mode 控制缩放策略；插值方式固定为双线性（bilinear）。
func Resize(src image.Image, w, h int, mode ResizeMode) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 || w <= 0 || h <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}

	var rw, rh int
	switch mode {
	case ResizeStretch:
		rw, rh = w, h
	case ResizeWidth:
		rw = w
		rh = int(math.Round(float64(w) * float64(sh) / float64(sw)))
	case ResizeHeight:
		rh = h
		rw = int(math.Round(float64(h) * float64(sw) / float64(sh)))
	case ResizeFill:
		rw, rh = fillDims(sw, sh, w, h)
	default: // ResizeFit
		rw, rh = fitDims(sw, sh, w, h)
	}
	if rw < 1 {
		rw = 1
	}
	if rh < 1 {
		rh = 1
	}

	return bilinearScale(src, rw, rh)
}

// Thumbnail 生成缩略图：等比缩放至填满宽高后居中裁剪。
func Thumbnail(src image.Image, w, h int) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 || w <= 0 || h <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	rw, rh := fillDims(sw, sh, w, h)
	scaled := bilinearScale(src, rw, rh)
	return Crop(scaled, w, h, AnchorCenter)
}

// Crop 从图片中裁剪指定宽高的区域。
func Crop(src image.Image, w, h int, anchor Anchor) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if w <= 0 || h <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	if w > sw {
		w = sw
	}
	if h > sh {
		h = sh
	}
	x, y := anchorXY(anchor, sw, sh, w, h)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, image.Point{x, y}, draw.Src)
	return dst
}

// Watermark 在图片上叠加图片水印。
// wm 为水印图；scale 为水印相对底图的比例 (0~1，默认 0.1)；
// opacity 为不透明度 (0~255，255 表示完全不透明)；anchor 为锚点位置。
func Watermark(src, wm image.Image, scale float64, opacity uint8,
	anchor Anchor, offsetX, offsetY int) image.Image {
	sb := src.Bounds()
	dst := image.NewNRGBA(sb)
	draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Src)

	wmb := wm.Bounds()
	if scale <= 0 {
		scale = 0.1
	}
	ww := int(math.Round(float64(wmb.Dx()) * scale))
	wh := int(math.Round(float64(wmb.Dy()) * scale))
	if ww < 1 {
		ww = 1
	}
	if wh < 1 {
		wh = 1
	}

	wmScaled := bilinearScale(wm, ww, wh)
	x, y := anchorXY(anchor, sb.Dx(), sb.Dy(), ww, wh)
	x += offsetX
	y += offsetY

	for dy := 0; dy < wh; dy++ {
		for dx := 0; dx < ww; dx++ {
			px := x + dx
			py := y + dy
			if px < 0 || px >= sb.Dx() || py < 0 || py >= sb.Dy() {
				continue
			}
			wc := wmScaled.NRGBAAt(dx, dy)
			if wc.A == 0 {
				continue
			}
			if opacity < 255 {
				wc.A = uint8(uint16(wc.A) * uint16(opacity) / 255)
			}
			off := dst.PixOffset(px, py)
			bc := dst.NRGBAAt(px, py)
			blendNRGBA((*[4]uint8)(dst.Pix[off:off+4:off+4]), bc, wc)
		}
	}
	return dst
}

// ------------------- bilinear scaling (stdlib only) -------------------

// bilinearScale 使用双线性插值缩放图片。
func bilinearScale(src image.Image, w, h int) *image.NRGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == w && sh == h {
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Src)
		return dst
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xRatio := float64(sw) / float64(w)
	yRatio := float64(sh) / float64(h)

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			// 源坐标（浮点）
			sx := (float64(dx) + 0.5)*xRatio - 0.5
			sy := (float64(dy) + 0.5)*yRatio - 0.5

			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1

			// 边界钳制
			if x0 < 0 {
				x0 = 0
			}
			if y0 < 0 {
				y0 = 0
			}
			if x1 >= sw {
				x1 = sw - 1
			}
			if y1 >= sh {
				y1 = sh - 1
			}

			// 插值权重
			xf := sx - float64(x0)
			yf := sy - float64(y0)

			i00 := toNRGBA(src.At(sb.Min.X+x0, sb.Min.Y+y0))
			i10 := toNRGBA(src.At(sb.Min.X+x1, sb.Min.Y+y0))
			i01 := toNRGBA(src.At(sb.Min.X+x0, sb.Min.Y+y1))
			i11 := toNRGBA(src.At(sb.Min.X+x1, sb.Min.Y+y1))

			off := dst.PixOffset(dx, dy)
			for c := 0; c < 4; c++ {
				v := lerp(
					lerp(float64(i00[c]), float64(i10[c]), xf),
					lerp(float64(i01[c]), float64(i11[c]), xf),
					yf,
				)
				dst.Pix[off+c] = uint8(math.Round(clamp(v, 0, 255)))
			}
		}
	}
	return dst
}

// toNRGBA 将颜色转换为 [4]uint8 NRGBA 数组。
func toNRGBA(c color.Color) [4]uint8 {
	r, g, b, a := c.RGBA()
	return [4]uint8{
		uint8(r >> 8),
		uint8(g >> 8),
		uint8(b >> 8),
		uint8(a >> 8),
	}
}

// lerp 线性插值。
func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// clamp 钳制值到 [lo, hi]。
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// blendNRGBA 将前景像素 fc 混合到背景像素 bc 上。
func blendNRGBA(dst *[4]uint8, bc, fc color.NRGBA) {
	a := uint16(fc.A)
	ia := 255 - a
	dst[0] = uint8((uint16(bc.R)*ia + uint16(fc.R)*a) / 255)
	dst[1] = uint8((uint16(bc.G)*ia + uint16(fc.G)*a) / 255)
	dst[2] = uint8((uint16(bc.B)*ia + uint16(fc.B)*a) / 255)
	dst[3] = uint8(min(int(bc.A)+int(fc.A), 255))
}

// ------------------- helpers -------------------

// fitDims 计算等比缩放后的尺寸（完整容纳）。
func fitDims(sw, sh, tw, th int) (int, int) {
	if sw == 0 || sh == 0 {
		return tw, th
	}
	ratio := math.Min(float64(tw)/float64(sw), float64(th)/float64(sh))
	return int(math.Round(float64(sw) * ratio)), int(math.Round(float64(sh) * ratio))
}

// fillDims 计算等比缩放后的尺寸（填满目标）。
func fillDims(sw, sh, tw, th int) (int, int) {
	if sw == 0 || sh == 0 {
		return tw, th
	}
	ratio := math.Max(float64(tw)/float64(sw), float64(th)/float64(sh))
	return int(math.Round(float64(sw) * ratio)), int(math.Round(float64(sh) * ratio))
}

// anchorXY 根据锚点计算子区域左上角位置。
func anchorXY(anchor Anchor, pw, ph, cw, ch int) (int, int) {
	switch anchor {
	case AnchorTopLeft:
		return 0, 0
	case AnchorTop:
		return (pw - cw) / 2, 0
	case AnchorTopRight:
		return pw - cw, 0
	case AnchorLeft:
		return 0, (ph - ch) / 2
	case AnchorRight:
		return pw - cw, (ph - ch) / 2
	case AnchorBottomLeft:
		return 0, ph - ch
	case AnchorBottom:
		return (pw - cw) / 2, ph - ch
	case AnchorBottomRight:
		return pw - cw, ph - ch
	default: // AnchorCenter
		return (pw - cw) / 2, (ph - ch) / 2
	}
}
