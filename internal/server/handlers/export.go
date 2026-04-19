package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"haulagex/internal/db"
	"haulagex/internal/models"
)

func AdminExportJobs(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from ต้องอยู่ในรูปแบบ YYYY-MM-DD"})
		return
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to ต้องอยู่ในรูปแบบ YYYY-MM-DD"})
		return
	}
	toEnd := toDate.Add(24 * time.Hour)

	var jobs []models.Job
	if err := db.DB.Where("scheduled_for >= ? AND scheduled_for < ?", fromDate, toEnd).
		Order("scheduled_for ASC, booking_no ASC").
		Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 12},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	f.SetCellValue(sheet, "A1", "Booking Number")
	f.SetCellValue(sheet, "B1", "รหัสตู้คอนเทนเนอร์")
	f.SetCellStyle(sheet, "A1", "B1", headerStyle)
	f.SetColWidth(sheet, "A", "A", 22)
	f.SetColWidth(sheet, "B", "B", 24)
	f.SetRowHeight(sheet, 1, 22)

	for i, j := range jobs {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), j.BookingNo)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), j.ContainerID)
	}

	filename := fmt.Sprintf("jobs_%s_%s.xlsx", from, to)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	f.Write(c.Writer)
}
