package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ===== MODEL =====
type Link struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"uniqueIndex;size:10"`
	OriginalURL string `gorm:"not null"`
	Visits      uint   `gorm:"default:0"`
	CreatedAt   time.Time
}

// ===== GLOBALS =====
var (
	db      *gorm.DB
	baseURL string
)

func main() {
	// Short URL base (hiện tại chạy ở :8081)
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	// DSN Postgres (đổi password/port nếu khác)
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=127.0.0.1 user=postgres password=123456 dbname=urlshortener port=5432 sslmode=disable TimeZone=Asia/Ho_Chi_Minh"
	}

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("cannot open database: ", err)
	}

	// Tự tạo bảng nếu thiếu
	if err := db.AutoMigrate(&Link{}); err != nil {
		log.Fatal("auto migrate failed: ", err)
	}

	// ===== ROUTER =====
	r := gin.Default()
	_ = r.SetTrustedProxies(nil) // bỏ cảnh báo trusted proxies khi dev

	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 1) Người dùng gửi URL dài -> tạo short URL + LƯU DB
	r.POST("/shorten", createShortURL)

	// 2) Redirect từ URL ngắn -> tăng visits + 302 redirect về link gốc
	r.GET("/:code", redirect)

	// 3) Theo dõi số lượt truy cập (stats) của 1 code
	r.GET("/stats/:code", stats)

	// 4) Liệt kê các link đã tạo (mới nhất trước)
	r.GET("/list", list)

	log.Println("listening on :8081")
	log.Fatal(r.Run(":8081"))
}

// ===== HANDLERS =====
type shortenReq struct {
	URL string `json:"url" binding:"required"`
}
type shortenResp struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func createShortURL(c *gin.Context) {
	var req shortenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if !isURL(req.URL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}

	// nếu URL này đã có code, trả lại luôn
	var existing Link
	if err := db.Where("original_url = ?", req.URL).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, shortenResp{Code: existing.Code, ShortURL: baseURL + "/" + existing.Code})
		return
	}

	code, err := genCodeUnique(6)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate code"})
		return
	}

	link := Link{Code: code, OriginalURL: req.URL}
	if err := db.Create(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusCreated, shortenResp{Code: link.Code, ShortURL: baseURL + "/" + link.Code})
}

func redirect(c *gin.Context) {
	code := c.Param("code")
	var link Link
	if err := db.Where("code = ?", code).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "code not found"})
		return
	}
	// tăng visits (không chặn redirect nếu update lỗi)
	if err := db.Model(&link).Update("visits", gorm.Expr("visits + 1")).Error; err != nil {
		log.Println("update visits error:", err)
	}
	c.Redirect(http.StatusFound, link.OriginalURL)
}

func stats(c *gin.Context) {
	code := c.Param("code")
	var link Link
	if err := db.Where("code = ?", code).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "code not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":         link.Code,
		"original_url": link.OriginalURL,
		"visits":       link.Visits,
		"created_at":   link.CreatedAt,
	})
}

func list(c *gin.Context) {
	var links []Link
	if err := db.Order("created_at desc").Limit(100).Find(&links).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, links)
}

// ===== HELPERS =====
const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func genCode(n int) (string, error) {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b[i] = alphabet[num.Int64()]
	}
	return string(b), nil
}

func genCodeUnique(n int) (string, error) {
	for i := 0; i < 8; i++ {
		code, err := genCode(n)
		if err != nil {
			return "", err
		}
		var cnt int64
		if err := db.Model(&Link{}).Where("code = ?", code).Count(&cnt).Error; err != nil {
			return "", err
		}
		if cnt == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate unique code")
}

func isURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}
