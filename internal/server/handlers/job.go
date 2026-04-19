package handlers

import (
	"haulagex/internal/db"
	"haulagex/internal/models"
	"haulagex/internal/server/middleware"
	"haulagex/internal/services"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AssignJobReq struct {
	BookingNo   string `json:"bookingNo" binding:"required"`
	ContainerID string `json:"containerId"`
	Origin      string `json:"origin" binding:"required"`
	Destination string `json:"destination" binding:"required"`
	AssigneeID  *uint  `json:"assigneeId"`
}

func AdminAssignJob(c *gin.Context) {
	var req AssignJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	originLoc, err := services.GeocodeAddress(req.Origin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Origin: " + err.Error()})
		return
	}
	destLoc, err := services.GeocodeAddress(req.Destination)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Destination: " + err.Error()})
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	j := models.Job{
		BookingNo:      strings.TrimSpace(req.BookingNo),
		ContainerID:    strings.ToUpper(strings.TrimSpace(req.ContainerID)),
		Origin:         strings.TrimSpace(req.Origin),
		Destination:    strings.TrimSpace(req.Destination),
		Status:         models.StatusAssigned,
		AssigneeID:     req.AssigneeID,
		ScheduledFor:   &today,
		OriginLat:      originLoc.Lat,
		OriginLng:      originLoc.Lng,
		DestinationLat: destLoc.Lat,
		DestinationLng: destLoc.Lng,
	}
	if err := db.DB.Create(&j).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create job"})
		return
	}
	c.JSON(http.StatusCreated, j)
}

func AdminListJobs(c *gin.Context) {
	var jobs []models.Job
	q := db.DB.Model(&models.Job{}).Preload("Assignee")

	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if aid := c.Query("assigneeId"); aid != "" {
		q = q.Where("assignee_id = ?", aid)
	}
	if search := strings.TrimSpace(c.Query("q")); search != "" {
		like := "%" + search + "%"
		q = q.Joins("LEFT JOIN users ON users.id = jobs.assignee_id").
			Where("jobs.booking_no ILIKE ? OR jobs.container_id ILIKE ? OR users.name ILIKE ?", like, like, like)
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	q = q.Limit(limit).Order("jobs.id DESC")

	if err := q.Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query error"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func ListMyJobs(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	var jobs []models.Job
	if err := db.DB.Where("assignee_id = ?", uid).Order("id DESC").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query error"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func ListOpenJobs(c *gin.Context) {
	var jobs []models.Job
	if err := db.DB.Where("assignee_id IS NULL AND status = ?", models.StatusAssigned).
		Order("id DESC").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query error"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func AcceptJob(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if j.Status != models.StatusAssigned {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job cannot be accepted in current status"})
		return
	}
	if j.AssigneeID != nil && *j.AssigneeID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "job assigned to another user"})
		return
	}
	j.AssigneeID = &uid
	j.Status = models.StatusAccepted
	if err := db.DB.Save(&j).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update error"})
		return
	}
	c.JSON(http.StatusOK, j)
}

type StartJobReq struct {
	QRText   string  `json:"qrText" binding:"required"`
	PhotoURL *string `json:"photoUrl"`
}

func StartJob(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if j.AssigneeID == nil || *j.AssigneeID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your job"})
		return
	}
	if j.Status != models.StatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job must be in 'accepted' to start"})
		return
	}

	var req StartJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	j.QRText = &req.QRText
	j.PhotoURL = req.PhotoURL
	// Mock OCR simple logic
	ocr := strings.ToUpper(strings.ReplaceAll(req.QRText, " ", ""))
	if len(ocr) > 11 {
		ocr = ocr[:11]
	}
	j.OCRText = &ocr
	j.StartedAt = &now
	j.Status = models.StatusInProgress

	if err := db.DB.Save(&j).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update error"})
		return
	}
	c.JSON(http.StatusOK, j)
}

type CompleteJobReq struct {
	// รับ JSON body เปล่าๆ ได้
}

func CompleteJob(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	// อนุญาตให้ complete ได้ถ้าเป็น in_progress หรือ accepted (เผื่อกรณีข้ามขั้นตอน)
	if j.Status != models.StatusInProgress && j.Status != models.StatusAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job must be 'in_progress' or 'accepted' to complete"})
		return
	}

	now := time.Now()
	j.CompletedAt = &now
	j.Status = models.StatusCompleted

	tx := db.DB.Begin()
	if err := tx.Save(&j).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update error"})
		return
	}

	// หาตำแหน่งล่าสุดของ Driver
	driverLocation, ok := GetUserLocation(uid)
	if !ok {
		// ถ้าไม่มีตำแหน่งล่าสุด ให้ใช้ตำแหน่งปลายทางของงานที่เพิ่งจบเป็นจุดเริ่มต้นแทน
		driverLocation = services.Location{Lat: j.DestinationLat, Lng: j.DestinationLng}
	}

	// หางานถัดไป
	nextJob := findNextJobForDriver(driverLocation)
	if nextJob != nil {
		nextJob.AssigneeID = &uid
		// ยังไม่เปลี่ยนสถานะเป็น accepted ให้คนขับกดรับเอง หรือจะ auto-accept ก็ได้
		// ในที่นี้แค่ assign ให้เห็น
		if err := tx.Save(nextJob).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "assign next job error"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db commit error"})
		return
	}

	response := gin.H{"status": "completed"}
	if nextJob == nil {
		response["next_job"] = "unavailable"
	} else {
		response["next_job_id"] = nextJob.ID
	}
	c.JSON(http.StatusOK, response)
}

func mockOCR(qrText string, photoURL *string) string {
	s := strings.ToUpper(qrText)
	s = strings.ReplaceAll(s, " ", "")
	if len(s) >= 7 {
		end := len(s)
		if end > 11 {
			end = 11
		}
		return s[:end]
	}
	if photoURL != nil && *photoURL != "" {
		u := *photoURL
		if len(u) > 11 {
			return strings.ToUpper(u[len(u)-11:])
		}
		return strings.ToUpper(u)
	}
	return "UNKNOWN"
}

func GetJob(c *gin.Context) {
	id := c.Param("id")
	var j models.Job
	if err := db.DB.Preload("Assignee").First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	roleAny, _ := c.Get(middleware.CtxUserRole)
	uid := c.GetUint(middleware.CtxUserID)
	role, _ := roleAny.(models.Role)

	if role != models.RoleAdmin {
		if j.AssigneeID != nil && *j.AssigneeID != uid {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}

	c.JSON(http.StatusOK, j)
}

type updateStatusReq struct {
	Status string `json:"status" binding:"required"`
}

func UpdateJobStatus(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if j.AssigneeID == nil || *j.AssigneeID != uid {
		if j.Status != models.StatusAssigned || j.AssigneeID != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "not your job"})
			return
		}
	}

	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Status {
	case string(models.StatusInProgress):
		now := time.Now()
		if j.Status == models.StatusAccepted {
			j.Status = models.StatusInProgress
			j.StartedAt = &now
		} else if j.Status == models.StatusAssigned {
			j.AssigneeID = &uid
			j.Status = models.StatusInProgress
			j.StartedAt = &now
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot start from current status"})
			return
		}
	case string(models.StatusCompleted):
		if j.Status != models.StatusInProgress {
			c.JSON(http.StatusBadRequest, gin.H{"error": "job must be in_progress to complete"})
			return
		}
		now := time.Now()
		j.Status = models.StatusCompleted
		j.CompletedAt = &now
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target status"})
		return
	}

	if err := db.DB.Save(&j).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update error"})
		return
	}
	c.JSON(http.StatusOK, j)
}

type BulkJobItem struct {
	BookingNo    string     `json:"bookingNo" binding:"required"`
	ContainerID  string     `json:"containerId" binding:"required"`
	Origin       string     `json:"origin" binding:"required"`
	Destination  string     `json:"destination" binding:"required"`
	ScheduledFor *time.Time `json:"scheduledFor"`
}
type BulkCreateReq struct {
	Jobs []BulkJobItem `json:"jobs" binding:"required,min=1,dive"`
}

func AdminBulkCreateJobs(c *gin.Context) {
	var req BulkCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var toCreate []models.Job
	now := time.Now()
	for _, it := range req.Jobs {
		sch := it.ScheduledFor
		if sch == nil {
			t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			sch = &t
		}
		originLoc, err := services.GeocodeAddress(it.Origin)
		if err != nil {
			log.Printf("[WARN] Skipping job '%s', Geocode failed for Origin '%s': %v", it.BookingNo, it.Origin, err)
			continue // ข้ามงานนี้ไป
		}
		destLoc, err := services.GeocodeAddress(it.Destination)
		if err != nil {
			log.Printf("[WARN] Skipping job '%s', Geocode failed for Destination '%s': %v", it.BookingNo, it.Destination, err)
			continue // ข้ามงานนี้ไป
		}

		toCreate = append(toCreate, models.Job{
			BookingNo:    strings.TrimSpace(it.BookingNo),
			ContainerID:  strings.ToUpper(strings.TrimSpace(it.ContainerID)),
			Origin:       strings.TrimSpace(it.Origin),
			Destination:  strings.TrimSpace(it.Destination),
			Status:       models.StatusAssigned,
			ScheduledFor: sch,
			// ⭐️ [แก้ไข] ใช้พิกัดที่ได้จาก Geocoding
			OriginLat:      originLoc.Lat,
			OriginLng:      originLoc.Lng,
			DestinationLat: destLoc.Lat,
			DestinationLng: destLoc.Lng,
		})
	}
	if err := db.DB.Create(&toCreate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create jobs"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"created": len(toCreate)})
}

type DistributeReq struct {
	Date *time.Time `json:"date"`
}

func AdminDistributeJobs(c *gin.Context) {
	var req DistributeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Date = nil
	}
	now := time.Now()

	targetDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if req.Date != nil {
		targetDate = time.Date(req.Date.Year(), req.Date.Month(), req.Date.Day(), 0, 0, 0, 0, now.Location())
	}

	var openJobs []models.Job
	if err := db.DB.
		Where("assignee_id IS NULL AND status = ? AND (scheduled_for IS NULL OR DATE(scheduled_for) <= DATE(?))", models.StatusAssigned, targetDate).
		Order("COALESCE(scheduled_for, 'epoch') ASC, id ASC").
		Find(&openJobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not query open jobs"})
		return
	}
	if len(openJobs) == 0 {
		c.JSON(http.StatusOK, gin.H{"assigned": 0, "message": "No open jobs to distribute."})
		return
	}

	var busyUserIDs []uint
	db.DB.Model(&models.Job{}).
		Where("status IN ?", []string{"assigned", "accepted", "in_progress"}).
		Pluck("DISTINCT assignee_id", &busyUserIDs)

	var availableUsers []models.User
	query := db.DB.Where("role = ?", models.RoleUser)
	if len(busyUserIDs) > 0 {
		query = query.Where("id NOT IN ?", busyUserIDs)
	}
	if err := query.Order("id ASC").Find(&availableUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not query available users"})
		return
	}
	if len(availableUsers) == 0 {
		c.JSON(http.StatusOK, gin.H{"assigned": 0, "message": "No available drivers found."})
		return
	}

	tx := db.DB.Begin()
	assignedCount := 0
	limit := len(openJobs)
	if len(availableUsers) < limit {
		limit = len(availableUsers)
	}

	for i := 0; i < limit; i++ {
		job := openJobs[i]
		user := availableUsers[i]
		if err := tx.Model(&job).Update("assignee_id", user.ID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign job to user " + user.Name})
			return
		}
		assignedCount++
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction commit error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"assigned": assignedCount})
}

func AdminAssignFirstJob(c *gin.Context) {
	today := time.Now()
	var openJobs []models.Job
	if err := db.DB.
		Where("assignee_id IS NULL AND status = ? AND DATE(scheduled_for) = DATE(?)", models.StatusAssigned, today).
		Find(&openJobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not query open jobs"})
		return
	}
	if len(openJobs) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "no open jobs for today"})
		return
	}

	var longestDistJob models.Job
	maxDist := -1
	hubLocation := services.Location{Lat: 13.7563, Lng: 100.5018}

	for _, job := range openJobs {
		destLocation := services.Location{Lat: job.DestinationLat, Lng: job.DestinationLng}
		dist, _ := services.EstimateDistanceDuration(hubLocation, destLocation)
		if dist > maxDist {
			maxDist = dist
			longestDistJob = job
		}
	}

	var user models.User
	if err := db.DB.Where("role = ?", models.RoleUser).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no available driver found"})
		return
	}

	longestDistJob.AssigneeID = &user.ID
	if err := db.DB.Save(&longestDistJob).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign job"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "first job assigned", "jobId": longestDistJob.ID, "assigneeId": user.ID})
}

func findNextJobForDriver(driverLocation services.Location) *models.Job {
	log.Println("--- [DEBUG] Starting findNextJobForDriver ---")
	now := time.Now()

	// ------------------------------------------------------------
	// ⭐️ [DEMO MODE] FORCED DRIVER LOCATION (OPTIONAL)
	// ปลด comment บรรทัดข้างล่างนี้ ถ้าต้องการบังคับให้คนขับอยู่ที่ "จุฬาฯ" เสมอตอนจบงาน
	// driverLocation = services.Location{Lat: 13.7384, Lng: 100.5321}
	// log.Printf("[DEMO] Forcing driver location to Chula: %v", driverLocation)
	// ------------------------------------------------------------

	// 1. หางานค้าง (Overdue) ก่อน
	var overdueJobs []models.Job
	db.DB.Where("assignee_id IS NULL AND status = ? AND DATE(scheduled_for) < DATE(?)", models.StatusAssigned, now).
		Order("scheduled_for ASC, id ASC").Find(&overdueJobs)
	if len(overdueJobs) > 0 {
		log.Printf("[DEBUG] Found %d overdue jobs. Assigning best one.", len(overdueJobs))
		return findBestJobInList(overdueJobs, driverLocation, now, true) // Overdue ไม่สนเวลาเลิกงาน
	}

	// 2. หางานวันนี้ (Today)
	var todaysOpenJobs []models.Job
	db.DB.Where("assignee_id IS NULL AND status = ? AND DATE(scheduled_for) = DATE(?)", models.StatusAssigned, now).
		Order("id ASC").Find(&todaysOpenJobs)
	if len(todaysOpenJobs) > 0 {
		log.Printf("[DEBUG] Found %d open jobs for today.", len(todaysOpenJobs))
		return findBestJobInList(todaysOpenJobs, driverLocation, now, false) // งานวันนี้ สนเวลาเลิกงาน
	}

	return nil
}

func findBestJobInList(jobs []models.Job, driverLocation services.Location, now time.Time, ignoreTimeCheck bool) *models.Job {
	// Real 20:00 cutoff in Thai timezone (UTC+7)
	bangkokTZ := time.FixedZone("Asia/Bangkok", 7*60*60)
	nowBKK := now.In(bangkokTZ)
	endOfWorkDay := time.Date(nowBKK.Year(), nowBKK.Month(), nowBKK.Day(), 20, 0, 0, 0, bangkokTZ)

	var bestJob *models.Job
	var minDuration time.Duration = -1

	for i := range jobs {
		job := jobs[i]
		originLoc := services.Location{Lat: job.OriginLat, Lng: job.OriginLng}
		destLoc := services.Location{Lat: job.DestinationLat, Lng: job.DestinationLng}

		// 1. คำนวณเวลาเดินทางจาก "จุดปัจจุบันของคนขับ" ไป "ต้นทางของงาน"
		_, travelToOrigin := services.GetRouteDuration(driverLocation, originLoc)

		if !ignoreTimeCheck {
			// 2. คำนวณเวลาทำงานจริง (ต้นทาง -> ปลายทาง)
			_, jobDuration := services.GetRouteDuration(originLoc, destLoc)

			// 3. เวลาที่คาดว่าจะเสร็จ = ตอนนี้ + เดินทางไปรับ + ทำงาน + เผื่อเวลา 30 นาที
			estimatedFinish := now.Add(travelToOrigin).Add(jobDuration).Add(30 * time.Minute)

			if estimatedFinish.After(endOfWorkDay) {
				log.Printf("[CUTOFF] Job #%d rejected. Est finish: %s > Cutoff 20:00", job.ID, estimatedFinish.In(bangkokTZ).Format("15:04"))
				continue
			}
		}

		// เลือกงานที่ใช้เวลาเดินทางไปรับน้อยที่สุด (ใกล้ที่สุด/เร็วที่สุด)
		if bestJob == nil || travelToOrigin < minDuration {
			minDuration = travelToOrigin
			bestJob = &job
		}
	}
	return bestJob
}

// AdminReassignAccidentJob — admin กด reassign เมื่อคนขับเกิดอุบัติเหตุ
// POST /api/admin/jobs/:id/accident-reassign
func AdminReassignAccidentJob(c *gin.Context) {
	id := c.Param("id")

	var j models.Job
	if err := db.DB.First(&j, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	if j.Status != models.StatusAccepted && j.Status != models.StatusInProgress {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only accepted or in_progress jobs can be accident-reassigned"})
		return
	}

	prevAssigneeID := j.AssigneeID

	// รีเซ็ตข้อมูลที่คนขับเดิมทำไปแล้ว (QR/OCR/ภาพ/เวลาเริ่ม)
	j.StartedAt = nil
	j.QRText = nil
	j.OCRText = nil
	j.PhotoURL = nil

	// หาคนขับที่ว่าง (ไม่มีงาน accepted/in_progress) ที่ไม่ใช่คนขับเดิม
	var busyIDs []uint
	db.DB.Model(&models.Job{}).
		Where("status IN ? AND assignee_id IS NOT NULL", []string{"accepted", "in_progress"}).
		Pluck("DISTINCT assignee_id", &busyIDs)

	var availableDrivers []models.User
	q := db.DB.Where("role = ?", models.RoleUser)
	if len(busyIDs) > 0 {
		q = q.Where("id NOT IN ?", busyIDs)
	}
	if prevAssigneeID != nil {
		q = q.Where("id != ?", *prevAssigneeID)
	}
	q.Find(&availableDrivers)

	tx := db.DB.Begin()

	if len(availableDrivers) == 0 {
		// ไม่มีคนขับว่าง → คืนเข้า stock
		j.AssigneeID = nil
		j.Status = models.StatusAssigned
		if err := tx.Save(&j).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		tx.Commit()
		c.JSON(http.StatusOK, gin.H{
			"reassigned": false,
			"message":    "no available driver — job returned to stock",
			"job":        j,
		})
		return
	}

	// หาคนขับที่ใกล้ต้นทางของงานมากที่สุด
	originLoc := services.Location{Lat: j.OriginLat, Lng: j.OriginLng}
	var bestDriver *models.User
	var minDuration time.Duration = -1
	for i := range availableDrivers {
		d := availableDrivers[i]
		driverLoc, ok := GetUserLocation(d.ID)
		if !ok {
			// ถ้าไม่รู้ตำแหน่งคนขับ ให้ใช้ origin job เป็น fallback (distance = 0)
			driverLoc = originLoc
		}
		_, dur := services.GetRouteDuration(driverLoc, originLoc)
		if bestDriver == nil || dur < minDuration {
			minDuration = dur
			bestDriver = &d
		}
	}

	j.AssigneeID = &bestDriver.ID
	j.Status = models.StatusAccepted
	if err := tx.Save(&j).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit error"})
		return
	}

	log.Printf("[ACCIDENT] Job #%d reassigned from driver #%v → driver #%d (%s)", j.ID, prevAssigneeID, bestDriver.ID, bestDriver.Name)
	c.JSON(http.StatusOK, gin.H{
		"reassigned":  true,
		"new_driver":  gin.H{"id": bestDriver.ID, "name": bestDriver.Name},
		"job":         j,
	})
}
