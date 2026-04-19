package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"haulagex/internal/models"
)

func ptr[T any](v T) *T { return &v }

type depot struct {
	Name string
	Lat  float64
	Lng  float64
}

var depots = []depot{
	{"SONIC (ลานตู้โซนิค ปิ่นทอง)", 13.131993, 101.011912},
	{"YJC (YJC Thailand Container Depot)", 13.125631, 101.012453},
	{"MCCT (MOL Container Center)", 13.124032, 101.004114},
	{"SMC1 (Smart Logistics LCB1)", 13.119668, 101.032264},
	{"Medlog (MTD PTH สาขาปิ่นทอง)", 13.126141, 101.019516},
	{"SRITAI (Srithai Depot LCB)", 13.123331, 100.982363},
	{"MPJ (MPJ Distribution Center)", 13.116632, 100.969973},
	{"CIMC (CIMC Logistics Service)", 13.098385, 100.988405},
	{"Thai Eng Kong (Orchid Depot LCB2)", 13.084402, 100.931539},
	{"KCTC2 / HK Depot", 13.058382, 100.919532},
	{"GFLCB1 (G-Fortune 1)", 13.087870, 100.931915},
	{"G Fortune 2 (LCB2)", 13.108527, 100.925307},
	{"G Fortune 3", 13.155455, 100.964247},
	{"G Fortune 4 (LCB4)", 13.067965, 100.915514},
	{"Siam Commercial Depot (ลานตู้ 1)", 13.127743, 100.912363},
	{"Siam Commercial Terminal", 13.128438, 100.901462},
	{"Siamcontainer Terminal (LCB Branch)", 13.057546, 100.996732},
	{"KSP Depot", 13.057241, 100.921540},
	{"Kerry Logistics (KLN Laem Chabang)", 13.117521, 100.977516},
	{"A-ONE Container Depot", 13.087175, 100.929004},
	{"HAST (Hutchison Ports)", 13.076000, 100.922000},
	{"KCTC2 (เคซีทีซี 2)", 13.059561, 100.919338},
}

var driverData = []struct {
	Name  string
	Phone string
	Plate string
}{
	{"สมชาย ใจดี", "0811111101", "กข 1001"},
	{"วิชัย มานะ", "0811111102", "กข 1002"},
	{"ประสิทธิ์ ศรีสุข", "0811111103", "กข 1003"},
	{"อนุชา พงษ์ไพบูลย์", "0811111104", "กข 1004"},
	{"ธนกร เจริญสุข", "0811111105", "กข 1005"},
	{"ชัยวัฒน์ บุญมี", "0811111106", "กข 1006"},
	{"สุรชัย ลาภมา", "0811111107", "กข 1007"},
	{"พิทักษ์ วงศ์สว่าง", "0811111108", "กข 1008"},
	{"นพดล ทองดี", "0811111109", "กข 1009"},
	{"ศักดิ์ชาย พรมมา", "0811111110", "กข 1010"},
	{"วรวุฒิ สุขสม", "0811111111", "กข 1011"},
	{"ภาณุวัฒน์ ดวงแก้ว", "0811111112", "กข 1012"},
	{"ชนาธิป ขันทอง", "0811111113", "กข 1013"},
	{"กฤษณะ มีสุข", "0811111114", "กข 1014"},
	{"ปิยะ รุ่งเรือง", "0811111115", "กข 1015"},
	{"เอกชัย ทวีศิลป์", "0811111116", "กข 1016"},
	{"สราวุธ คงดี", "0811111117", "กข 1017"},
	{"ณัฐพล สมบูรณ์", "0811111118", "กข 1018"},
	{"วุฒิพงษ์ แก้วมณี", "0811111119", "กข 1019"},
	{"ทวีศักดิ์ พันธุ์ดี", "0811111120", "กข 1020"},
	{"อภิชาติ สุวรรณ", "0811111121", "กข 1021"},
	{"ปรีชา บุญเรือง", "0811111122", "กข 1022"},
}

// containerIDs ตรงกับรูปใน uploads/seed_BK{i+1:04d}-TH.jpg ทุกใบ
var containerIDs = []string{
	"MRKU6309407", "MTMU2891799", "JHSU2661065", "TRHU1362723", "GAOU2534602",
	"REGU5228753", "EMCU6209171", "EGHU3635081", "FCIU7073293", "DRYU2766873",
	"EGHU3560547", "CAIU5802037", "FCIU2011302", "UESU3002469", "MSKU4280420",
	"CICU6785073", "BMOU1686915", "TGCU2024823", "MSNU1473760", "MSNU3835127",
	"SEKU1127061", "NSSU0195318", "TGBU2320436", "OOLU0365990", "GAOU6006964",
	"TRHU2705473", "FTAU1604860", "DPWU2097289", "OOLU1839990", "JXLU4468432",
	"TGBU8562838", "ONEU2253811", "TCLU4665276", "TIIU4441610", "TIIU6836266",
	"TRHU1181411", "EGHU3177482", "EITU0123204", "EGHU3664177", "EISU2290465",
	"SEKU1028140", "EGHU3802677", "NBYU0320080", "NBYU0322333", "NYKU3770220",
	"NYKU3862097", "HALU2503978", "SKLU1920875", "TCLU4673630", "TCLU4673630",
	"FTAU2422999", "TIIU2518987", "EITU0566630", "EMCU6331364", "KKTU8142339",
	"NYKU9767008", "WHSU2839607", "WHSU2935853", "CCLU7721525", "FFAU6563010",
	"MSMU8650586", "REGU5236717", "ONEU2083989", "TLLU3494578", "EGSU3903974",
	"DRYU2664716", "EGHU3197030", "EMCU6364033", "MRKU9899225", "SKHU9969913",
	"SIKU3090478", "DFSU7718565", "PHRU2203002", "CAIU6951478", "NYKU9741425",
	"TRHU1690070", "MSKU3255530", "MRKU6532283", "MSMU3751470", "MSMU1726329",
	"UETU2555709", "OOCU0766698", "DPWU2130690", "TCNU5547932", "OOLU1824223",
	"SELU4298701", "DPWU2206348", "DPWU2085884", "EITU0600837", "EGHU3681549",
	"EMCU6120211", "DRYU2665517", "EITU0213247", "EGHU3287509", "EITU3021335",
	"HALU2424992", "HALU2512331", "EGHU3946648", "EGHU3861978", "TXGU6742197",
	// BK0101–BK0126: วนซ้ำรูป 1–26 (ตู้เดิม เที่ยวต่างวัน)
	"MRKU6309407", "MTMU2891799", "JHSU2661065", "TRHU1362723", "GAOU2534602",
	"REGU5228753", "EMCU6209171", "EGHU3635081", "FCIU7073293", "DRYU2766873",
	"EGHU3560547", "CAIU5802037", "FCIU2011302", "UESU3002469", "MSKU4280420",
	"CICU6785073", "BMOU1686915", "TGCU2024823", "MSNU1473760", "MSNU3835127",
	"SEKU1127061", "NSSU0195318", "TGBU2320436", "OOLU0365990", "GAOU6006964",
	"TRHU2705473",
}

// djc = driver job count สำหรับแต่ละวัน
type djc struct {
	driverIdx int
	numJobs   int
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=haulagex port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("connect DB: %v", err)
	}

	// ── สร้างคนขับ 22 คน ─────────────────────────────────────────────────────
	hash, _ := bcrypt.GenerateFromPassword([]byte("driver123"), bcrypt.DefaultCost)
	ph := string(hash)
	userIDs := make([]uint, len(driverData))
	for i, d := range driverData {
		u := models.User{
			Name:         d.Name,
			Phone:        d.Phone,
			Email:        fmt.Sprintf("driver%d@hx.local", i+1),
			PasswordHash: ph,
			Role:         models.RoleUser,
			LicensePlate: d.Plate,
		}
		db.Where(models.User{Email: u.Email}).FirstOrCreate(&u)
		userIDs[i] = u.ID
		log.Printf("✓ driver%-2d  %s  (ID=%d)", i+1, u.Name, u.ID)
	}

	loc, _ := time.LoadLocation("Asia/Bangkok")

	// ── สร้าง route pool: ทุก (orig, dest) ที่ไม่ซ้ำกัน, สุ่มลำดับ ───────────
	type route struct{ orig, dest int }
	var routePool []route
	for o := range depots {
		for d := range depots {
			if o != d {
				routePool = append(routePool, route{o, d})
			}
		}
	}
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(routePool), func(i, j int) {
		routePool[i], routePool[j] = routePool[j], routePool[i]
	})
	ri := 0
	nextRoute := func() route {
		r := routePool[ri%len(routePool)]
		ri++
		return r
	}

	ci := 0
	nextCID := func() string {
		c := containerIDs[ci%len(containerIDs)]
		ci++
		return c
	}

	// ── แผนงานต่อวัน ──────────────────────────────────────────────────────────
	//
	// แต่ละรายการ = (driverIdx, จำนวนเที่ยว)
	// สลับกันว่าใครทำ 1 เที่ยว ใครทำ 2 เที่ยวในแต่ละวัน
	//
	// 15/3 (เสาร์) — ทำงานระดับกลาง: 8 คนทำ 2 เที่ยว, 14 คนทำ 1 เที่ยว → 30 งาน
	// 16/3 (อาทิตย์) — งานเยอะขึ้น: สลับกัน 11 คนทำ 2 เที่ยว, 11 คนทำ 1 เที่ยว → 33 งาน
	// 17/3 (จันทร์)  — งานหนักสุด: 12 คนทำ 2 เที่ยว, 10 คนทำ 1 เที่ยว → 34 งาน
	// 18/3 (อังคาร) — เบาลงหน่อย: 7 คนทำ 2 เที่ยว, 15 คนทำ 1 เที่ยว → 29 งาน

	type dayPlan struct {
		day     time.Time
		drivers []djc
	}

	plans := []dayPlan{
		{
			time.Date(2026, 3, 15, 0, 0, 0, 0, loc),
			[]djc{
				{0, 2}, {1, 1}, {2, 2}, {3, 1}, {4, 2}, {5, 1},
				{6, 2}, {7, 1}, {8, 2}, {9, 1}, {10, 2}, {11, 1},
				{12, 2}, {13, 1}, {14, 2}, {15, 1}, {16, 1}, {17, 1},
				{18, 1}, {19, 1}, {20, 1}, {21, 1},
			},
		},
		{
			time.Date(2026, 3, 16, 0, 0, 0, 0, loc),
			[]djc{
				{0, 1}, {1, 2}, {2, 1}, {3, 2}, {4, 1}, {5, 2},
				{6, 1}, {7, 2}, {8, 1}, {9, 2}, {10, 1}, {11, 2},
				{12, 1}, {13, 2}, {14, 1}, {15, 2}, {16, 1}, {17, 2},
				{18, 1}, {19, 2}, {20, 1}, {21, 2},
			},
		},
		{
			time.Date(2026, 3, 17, 0, 0, 0, 0, loc),
			[]djc{
				{0, 2}, {1, 2}, {2, 1}, {3, 2}, {4, 1}, {5, 2},
				{6, 1}, {7, 2}, {8, 1}, {9, 2}, {10, 1}, {11, 2},
				{12, 1}, {13, 2}, {14, 1}, {15, 2}, {16, 1}, {17, 2},
				{18, 1}, {19, 2}, {20, 1}, {21, 2},
			},
		},
		{
			time.Date(2026, 3, 18, 0, 0, 0, 0, loc),
			[]djc{
				{0, 1}, {1, 1}, {2, 2}, {3, 1}, {4, 1}, {5, 2},
				{6, 1}, {7, 1}, {8, 2}, {9, 1}, {10, 1}, {11, 2},
				{12, 1}, {13, 1}, {14, 2}, {15, 1}, {16, 1}, {17, 2},
				{18, 1}, {19, 1}, {20, 2}, {21, 1},
			},
		},
	}

	// ── สร้างงานทั้งหมด ───────────────────────────────────────────────────────
	bk := 1
	created := 0

	for _, dp := range plans {
		dayTotal := 0
		for _, d := range dp.drivers {
			dayTotal += d.numJobs
		}
		log.Printf("\n── %s (%d งาน) ──────────────────────────────",
			dp.day.Format("02/01/2006"), dayTotal)

		for _, d := range dp.drivers {
			// เวลาเริ่มงานแรกของแต่ละคน: 06:00–08:20 (สลับกันไม่ให้ออกพร้อมกัน)
			startMin := 6*60 + (d.driverIdx%8)*20

			for jobNum := 0; jobNum < d.numJobs; jobNum++ {
				// ระยะเวลาแต่ละเที่ยว: 90–180 นาที (ขึ้นกับคนขับ + เที่ยวที่)
				durMin := 90 + (d.driverIdx*13+jobNum*31)%91

				r := nextRoute()
				cid := nextCID()

				start := dp.day.Add(time.Duration(startMin) * time.Minute)
				end := start.Add(time.Duration(durMin) * time.Minute)

				// พัก 30–60 นาทีก่อนเที่ยวถัดไป
				startMin += durMin + 30 + (d.driverIdx%4)*10

				bkNo := fmt.Sprintf("BK%04d-TH", bk)
				dayTime := dp.day

				job := models.Job{
					BookingNo:      bkNo,
					ContainerID:    cid,
					Origin:         depots[r.orig].Name,
					Destination:    depots[r.dest].Name,
					OriginLat:      depots[r.orig].Lat,
					OriginLng:      depots[r.orig].Lng,
					DestinationLat: depots[r.dest].Lat,
					DestinationLng: depots[r.dest].Lng,
					Status:         models.StatusCompleted,
					ScheduledFor:   &dayTime,
					AssigneeID:     &userIDs[d.driverIdx],
					StartedAt:      &start,
					CompletedAt:    &end,
					OCRText:        ptr(cid),
					QRText:         ptr(cid),
					PhotoURL:       ptr(fmt.Sprintf("/uploads/seed_%s.jpg", bkNo)),
				}

				if err := db.Create(&job).Error; err != nil {
					log.Printf("  skip %s: %v", bkNo, err)
					bk++
					continue
				}

				log.Printf("  ✓ %s | %s → %s | %s | %s–%s",
					bkNo,
					depots[r.orig].Name,
					depots[r.dest].Name,
					driverData[d.driverIdx].Name,
					start.Format("15:04"),
					end.Format("15:04"),
				)
				created++
				bk++
			}
		}
	}

	log.Printf("\n✅ สร้างงานเสร็จ %d งาน", created)
	log.Printf("📊 สรุป:")
	log.Printf("   15/3 (เสาร์)  : 30 งาน — ทุกคนขับทำงาน ส่วนใหญ่ 1 เที่ยว บางคน 2 เที่ยว")
	log.Printf("   16/3 (อาทิตย์): 33 งาน — งานเยอะขึ้น สลับกันทำ 1–2 เที่ยว")
	log.Printf("   17/3 (จันทร์) : 34 งาน — งานหนักสุด ส่วนใหญ่ 2 เที่ยว")
	log.Printf("   18/3 (อังคาร) : 29 งาน — เบาลงหน่อย บางคนทำ 2 เที่ยว")
	log.Printf("   👤 22 คนขับ | email: driver1-22@hx.local | password: driver123")
	log.Printf("   🗺  %d เส้นทางไม่ซ้ำกัน จากทั้งหมด %d คู่ที่เป็นไปได้",
		created, len(depots)*(len(depots)-1))
}
