// สร้าง placeholder JPEG สำหรับ seed data ทั้ง 126 งาน
// รัน: go run cmd/genimages/main.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math"
	"os"
)

func hsvToRGB(h, s, v float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360)
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255)
}

func main() {
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatal(err)
	}

	const W, H = 320, 240
	const total = 126

	for n := 1; n <= total; n++ {
		bkNo := fmt.Sprintf("BK%04d-TH", n)
		filename := fmt.Sprintf("uploads/seed_%s.jpg", bkNo)

		// hue สลับไปตาม index: เหมือนสีตู้คอนเทนเนอร์จริง (ฟ้า เขียว แดง ส้ม เทา)
		hue := math.Mod(float64(n)*47.3, 360)
		sat := 0.55 + math.Mod(float64(n)*0.07, 0.3) // 0.55–0.85
		val := 0.45 + math.Mod(float64(n)*0.05, 0.25) // 0.45–0.70

		baseR, baseG, baseB := hsvToRGB(hue, sat, val)

		img := image.NewNRGBA(image.Rect(0, 0, W, H))

		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				var r, g, b uint8

				// ──  พื้นหลัง: gradient แนวตั้ง เลียนแบบสีตู้
				shade := 1.0 - float64(y)/float64(H)*0.35
				r = uint8(float64(baseR) * shade)
				g = uint8(float64(baseG) * shade)
				b = uint8(float64(baseB) * shade)

				// ── ลายแนวนอน: ribs ของตู้คอนเทนเนอร์
				rib := (y / 18) % 2
				if rib == 1 {
					r = clamp(int(r) - 18)
					g = clamp(int(g) - 18)
					b = clamp(int(b) - 18)
				}

				// ── ขอบซ้าย-ขวา: โครงเหล็ก
				if x < 10 || x > W-11 {
					r = clamp(int(r) - 35)
					g = clamp(int(g) - 35)
					b = clamp(int(b) - 35)
				}

				// ── แถบป้ายขาวกลางตู้
				if y >= H/2-18 && y <= H/2+18 {
					blend := 0.82
					r = uint8(float64(r)*(1-blend) + 240*blend)
					g = uint8(float64(g)*(1-blend) + 240*blend)
					b = uint8(float64(b)*(1-blend) + 240*blend)
				}

				img.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
			}
		}

		f, err := os.Create(filename)
		if err != nil {
			log.Fatalf("create %s: %v", filename, err)
		}
		if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 72}); err != nil {
			log.Fatalf("encode %s: %v", filename, err)
		}
		f.Close()
		log.Printf("✓ %s", filename)
	}

	log.Printf("\n✅ สร้างรูปเสร็จ %d ไฟล์ ใน uploads/", total)
	log.Printf("   ขั้นต่อไป: git add uploads/ && git commit && git push")
}

func clamp(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
