// run_ocr.go (เวอร์ชันอัปเกรดที่คำนวณ CER)
package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// --- Start: คัดลอกมาจาก backend/internal/server/handlers/ocr.go ---
var containerRe = regexp.MustCompile(`([A-Z]{4}[\s\-_]*\d{3}[\s\-_]*\d{4})`)
var lookLikeDigit = map[rune]rune{'O': '0', 'Q': '0', 'I': '1', 'L': '1', 'Z': '2', 'S': '5', 'B': '8'}
var lookLikeLetter = map[rune]rune{'0': 'O', '1': 'I', '2': 'Z', '5': 'S', '8': 'B'}

func forceContainerPattern(s string) string {
	u := strings.ToUpper(s)
	buf := make([]rune, 0, len(u))
	for _, r := range u {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			buf = append(buf, r)
		}
	}
	out := make([]rune, 0, 11)
	for i := 0; i < 4 && i < len(buf); i++ {
		r := buf[i]
		if r >= '0' && r <= '9' {
			if rr, ok := lookLikeLetter[r]; ok {
				r = rr
			} else {
				r = 'A' //
			}
		}
		out = append(out, r)
	}
	for i := 4; len(out) < 11 && i < len(buf); i++ {
		r := buf[i]
		if r >= 'A' && r <= 'Z' {
			if rr, ok := lookLikeDigit[r]; ok {
				r = rr
			} else {
				r = '0' //
			}
		}
		out = append(out, r)
	}
	if len(out) == 11 {
		return string(out)
	}
	return ""
}

func runTesseractCLI(imgPath string, psm string) (string, error) {
	args := []string{imgPath, "stdout", "-l", "eng", "--psm", psm}
	args = append(args, "-c", "tessedit_char_whitelist=ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_ ") //
	cmd := exec.Command("tesseract", args...)
	cmd.Env = os.Environ()
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract run error: %v: %s", err, errb.String())
	}
	return out.String(), nil
}

func tryTesseract(imgPath string) (raw string, normalized string, err error) {
	psms := []string{"7", "6", "11"} //
	var rawLogs strings.Builder
	for _, p := range psms {
		txt, e := runTesseractCLI(imgPath, p)
		if e != nil {
			err = e
			rawLogs.WriteString(fmt.Sprintf("PSM %s Error: %v\n", p, e))
			continue
		}
		rawLogs.WriteString(fmt.Sprintf("PSM %s Raw:\n%s\n---\n", p, txt))
		raw = rawLogs.String()
		up := strings.ToUpper(txt)
		if m := containerRe.FindString(up); m != "" {
			if n := forceContainerPattern(m); n != "" {
				return raw, n, nil
			}
		}
		if n := forceContainerPattern(up); n != "" {
			return raw, n, nil
		}
	}
	return raw, "", err
}

// --- End: คัดลอกจาก ocr.go ---

// --- ⭐️ [ฟังก์ชันใหม่] คำนวณ Levenshtein Distance (จำเป็นสำหรับ CER) ---
func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
	}
	for i := 0; i <= n; i++ {
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[n][m]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	}
	if b < c {
		return b
	}
	return c
}

// ⭐️ [ฟังก์ชันใหม่] คำนวณ CER (0.0 ถึง 1.0)
func calculateCER(groundTruth, prediction string) float64 {
	truthLen := utf8.RuneCountInString(groundTruth)
	if truthLen == 0 {
		return 0.0 // หลีกเลี่ยงการหารด้วย 0
	}
	distance := levenshteinDistance(groundTruth, prediction)
	return float64(distance) / float64(truthLen)
}

// --- Main Test Script ---

const (
	GroundTruthFile = "ground_truth.csv"
	ResultsFile     = "ocr_results.csv"
	ImageDir        = "uploads"
)

type ResultRow struct {
	Filename    string
	GroundTruth string
	Prediction  string
	RawOCR      string
	CER         string // ⭐️ [คอลัมน์ใหม่]
}

func main() {
	f, err := os.Open(GroundTruthFile)
	if err != nil {
		log.Fatalf("ERROR: ไม่สามารถเปิดไฟล์ %s. กรุณาสร้างไฟล์นี้ก่อน\nError: %v", GroundTruthFile, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	header, err := csvReader.Read()
	if err != nil {
		log.Fatalf("ERROR: ไม่สามารถอ่าน header จาก %s: %v", GroundTruthFile, err)
	}

	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.ToLower(strings.TrimSpace(h))] = i
	}
	filenameIdx, okF := colMap["filename"]
	truthIdx, okT := colMap["ground_truth"]
	if !okF || !okT {
		log.Fatalf("ERROR: ไฟล์ CSV ต้องมีคอลัมน์ 'filename' และ 'ground_truth'")
	}

	var results []ResultRow
	results = append(results, ResultRow{
		Filename:    "filename",
		GroundTruth: "ground_truth",
		Prediction:  "prediction",
		CER:         "CER", // ⭐️ [คอลัมน์ใหม่]
		RawOCR:      "raw_ocr_output",
	})

	log.Println("--- เริ่มต้นประมวลผล OCR (พร้อมคำนวณ CER) ---")

	var processedCount, matchCount int
	var totalCER float64 // ⭐️ [ตัวแปรใหม่]

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("WARN: ข้ามแถวที่มีปัญหา: %v", err)
			continue
		}

		filename := record[filenameIdx]
		groundTruth := record[truthIdx]
		imgPath := filepath.Join(ImageDir, filename)

		if _, err := os.Stat(imgPath); os.IsNotExist(err) {
			log.Printf("WARN: ไม่พบไฟล์รูปภาพ, ข้าม: %s", imgPath)
			// ... (ข้ามการบันทึกแถวที่ไม่มีไฟล์) ...
			continue
		}

		log.Printf("Processing: %s", filename)
		raw, norm, _ := tryTesseract(imgPath) // เรียกใช้ฟังก์ชัน OCR

		// ⭐️ [คำนวณ CER]
		cer := calculateCER(groundTruth, norm)
		totalCER += cer

		results = append(results, ResultRow{
			Filename:    filename,
			GroundTruth: groundTruth,
			Prediction:  norm,
			CER:         fmt.Sprintf("%.2f%%", cer*100), // ⭐️ บันทึก CER เป็น %
			RawOCR:      raw,
		})

		processedCount++
		if norm == groundTruth && norm != "" {
			matchCount++
		}
	}

	log.Println("--- ประมวลผลเสร็จสิ้น, กำลังเขียนผลลัพธ์... ---")

	outFile, err := os.Create(ResultsFile)
	if err != nil {
		log.Fatalf("ERROR: ไม่สามารถสร้างไฟล์ %s: %v", ResultsFile, err)
	}
	defer outFile.Close()

	csvWriter := csv.NewWriter(outFile)
	for _, row := range results {
		// ⭐️ [อัปเดตการเขียน CSV]
		csvWriter.Write([]string{row.Filename, row.GroundTruth, row.Prediction, row.CER, row.RawOCR})
	}
	csvWriter.Flush()

	log.Printf("เขียนผลลัพธ์ %d รายการลงใน %s เรียบร้อยแล้ว\n", len(results)-1, ResultsFile)

	// 4. สรุปผล Accuracy (ทั้งสองแบบ)
	if processedCount > 0 {
		accuracy := (float64(matchCount) / float64(processedCount)) * 100
		avgCER := (totalCER / float64(processedCount)) * 100 // ⭐️ [คำนวณ CER เฉลี่ย]

		fmt.Println("\n--- 📊 รายงานความแม่นยำ (Accuracy Report) ---")
		fmt.Printf("รูปภาพทั้งหมดที่ประมวลผล: %d\n", processedCount)
		fmt.Println("---")
		fmt.Printf("1. Exact Match Accuracy (โหด): %.2f%%\n", accuracy)
		fmt.Printf("   (อ่านถูกเป๊ะ %d รูป จาก %d รูป)\n", matchCount, processedCount)
		fmt.Println("---")
		fmt.Printf("2. Average CER (ละเอียด): %.2f%%\n", avgCER)
		fmt.Printf("   (ยิ่งน้อยยิ่งดี - 0%% คือดีที่สุด)\n")
		fmt.Println("-------------------------------------------")

	} else {
		fmt.Println("ไม่พบข้อมูลรูปภาพที่จะประมวลผล")
	}
}
