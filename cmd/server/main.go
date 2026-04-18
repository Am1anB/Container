package main

import (
	"fmt"
	"haulagex/internal/auth"
	"haulagex/internal/config"
	"haulagex/internal/db"
	"haulagex/internal/server"
	"io"
	"log"
	"net/http"
	"os"
)

// (ฟังก์ชันตรวจสอบ IP - ไม่ต้องแก้ไข)
func printMyPublicIP() {
	log.Println("[INFO] Checking Public IP Address to use for Nostra Whitelist...")
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		log.Printf("[ERROR] Could not get public IP: %v", err)
		log.Println("[WARN] Please find your server's Public IP manually and add it to Nostra Console.")
		return
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ERROR] Could not read public IP response: %v", err)
		return
	}

	log.Println("==============================================================")
	log.Printf("[DEBUG] My Public IP (for Nostra Whitelist) is: %s", string(ip))
	log.Println("==============================================================")
}

func main() {
	printMyPublicIP()

	cfg := config.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// ถ้าไม่มี (รัน Local) ค่อยใช้ค่าแยกๆ เหมือนเดิม
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
		)
	}

	db.Connect(dsn)
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate error: %v", err)
	}
	if err := db.SeedAdmin(); err != nil {
		log.Fatalf("seed admin error: %v", err)
	}

	jwtSvc := auth.NewJWTService(
		cfg.JWTAccessSecret,
		cfg.JWTRefreshSecret,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
	)

	// ⭐️ 2. ดึงค่า PORT จาก Environment Variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082" // ⭐️ ถ้าไม่เจอ (ตอนรัน local) ให้ใช้ 8082 เหมือนเดิม
	}

	srv := server.New(port, jwtSvc)                        // ⭐️ 3. ใช้ port ที่ได้มา
	log.Printf("[INFO] Server starting on port: %s", port) // (เพิ่ม Log ให้เห็นชัดๆ)
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
