package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"haulagex/internal/db"
	"haulagex/internal/models"
	"haulagex/internal/server/middleware"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ensureDir สร้างโฟลเดอร์ถ้ายังไม่มี
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return nil
}

// containerRe จับ pattern ตู้ container ที่มี separator ได้หลายรูปแบบ
var containerRe = regexp.MustCompile(`[A-Z]{4}[\s\-_.]*(?:\d[\s\-_.]*){6}\d`)

var lookLikeDigit = map[rune]rune{
	'O': '0', 'Q': '0', 'D': '0',
	'I': '1', 'L': '1',
	'Z': '2',
	'S': '5',
	'G': '6',
	'B': '8',
}
var lookLikeLetter = map[rune]rune{
	'0': 'O', '1': 'I', '2': 'Z', '5': 'S', '8': 'B',
}

// forceContainerPattern แก้ OCR error ให้เป็นรูปแบบตู้คอนเทนเนอร์ (AAAA1234567)
func forceContainerPattern(s string) string {
	u := strings.ToUpper(s)
	buf := make([]rune, 0, len(u))
	for _, r := range u {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			buf = append(buf, r)
		}
	}
	if len(buf) < 11 {
		return ""
	}
	out := make([]rune, 0, 11)
	for i := 0; i < 4; i++ {
		r := buf[i]
		if r >= '0' && r <= '9' {
			if rr, ok := lookLikeLetter[r]; ok {
				r = rr
			} else {
				r = 'A'
			}
		}
		out = append(out, r)
	}
	for i := 4; len(out) < 11; i++ {
		r := buf[i]
		if r >= 'A' && r <= 'Z' {
			if rr, ok := lookLikeDigit[r]; ok {
				r = rr
			} else {
				r = '0'
			}
		}
		out = append(out, r)
	}
	return string(out)
}

// extractContainer พยายามหา container number จาก text โดยใช้ regex ก่อน แล้ว fallback force
func extractContainer(text string) string {
	up := strings.ToUpper(text)
	if m := containerRe.FindString(up); m != "" {
		if n := forceContainerPattern(m); n != "" {
			return n
		}
	}
	// Fallback: ถ้า text มีความยาวพอ ให้ force ทั้งก้อน
	if n := forceContainerPattern(up); n != "" {
		return n
	}
	return ""
}

type ocrRequest struct {
	ImageB64 string `json:"image_b64"`
}

type OCRCandidate struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

type OCRResponse struct {
	Text       string         `json:"text"`
	Raw        any            `json:"raw"`
	Candidates []OCRCandidate `json:"candidates"`
}

func ocrServiceURL() string {
	if u := os.Getenv("OCR_SERVICE_URL"); u != "" {
		return u
	}
	return "http://localhost:8001"
}

func callPaddleOCR(absPath string) (*OCRResponse, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read image failed: %v", err)
	}

	payload := ocrRequest{ImageB64: base64.StdEncoding.EncodeToString(data)}
	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post(ocrServiceURL()+"/predict", "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("connect to OCR service failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR service error: %d", resp.StatusCode)
	}

	var result OCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// OCRJob ฟังก์ชันหลักที่รับรูปจาก User
func OCRJob(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	roleAny, _ := c.Get(middleware.CtxUserRole)
	role, _ := roleAny.(models.Role)
	if role != models.RoleAdmin {
		if j.AssigneeID == nil || *j.AssigneeID != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	uploadDir := filepath.Join(".", "uploads")
	if err := ensureDir(uploadDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create upload dir"})
		return
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("job_%s_%d%s", id, time.Now().UnixNano(), ext)
	fullPath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save file error"})
		return
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		absPath = fullPath
	}

	ocrResp, err := callPaddleOCR(absPath)
	rawText := ""
	if err != nil {
		fmt.Printf("OCR Error: %v\n", err)
	} else {
		rawText = ocrResp.Text
	}

	// ลอง extract จาก candidates แต่ละบรรทัดก่อน (เรียงตาม confidence แล้วจาก Python)
	normalized := ""
	if ocrResp != nil {
		for _, c := range ocrResp.Candidates {
			if n := extractContainer(c.Text); n != "" {
				normalized = n
				break
			}
		}
	}

	// Fallback: ลองจาก full text ที่ Python ต่อไว้
	if normalized == "" && rawText != "" {
		normalized = extractContainer(rawText)
	}

	photoURL := "/uploads/" + filename
	j.PhotoURL = &photoURL
	if strings.TrimSpace(normalized) != "" {
		j.OCRText = &normalized
	}

	if err := db.DB.Save(&j).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update job error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"text":     normalized,
		"raw":      rawText,
		"photoUrl": photoURL,
	})
}

/*python ocr_service/main.py*/
