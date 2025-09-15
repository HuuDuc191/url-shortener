package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	connStr := "postgres://postgres:123456@localhost:5432/huuducgo?sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Không kết nối được DB:", err)
	}
	defer db.Close()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Nhập URL dài: ")
	longURL, _ := reader.ReadString('\n')
	longURL = longURL[:len(longURL)-1]

	shortID := generateID(6)
	_, err = db.Exec("INSERT INTO urls (short_id, original_url) VALUES ($1, $2)", shortID, longURL)
	if err != nil {
		log.Fatal("Lỗi khi lưu DB:", err)
	}
	fmt.Printf("Tạo thành công!\nURL gốc: %s\nURL ngắn: http://localhost:8081/%s\n", longURL, shortID)

	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/", redirectHandler)

	log.Println("Server chạy tại http://localhost:8081 ...")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[1:]
	if id == "" {
		fmt.Fprintln(w, "Chào mừng đến URL Shortener! Gửi POST /shorten với JSON {\"url\": \"https://...\"}")
		return
	}

	var original string
	err := db.QueryRow("SELECT original_url FROM urls WHERE short_id=$1", id).Scan(&original)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Lỗi DB", http.StatusInternalServerError)
		return
	}

	_, _ = db.Exec("UPDATE urls SET clicks = clicks + 1 WHERE short_id=$1", id)

	http.Redirect(w, r, original, http.StatusFound)
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	ShortURL string `json:"short_url"`
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Chỉ hỗ trợ POST", http.StatusMethodNotAllowed)
		return
	}

	var req ShortenRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.URL == "" {
		http.Error(w, "Dữ liệu không hợp lệ", http.StatusBadRequest)
		return
	}

	shortID := generateID(6)

	_, err = db.Exec("INSERT INTO urls (short_id, original_url) VALUES ($1, $2)", shortID, req.URL)
	if err != nil {
		http.Error(w, "Lỗi lưu DB", http.StatusInternalServerError)
		return
	}

	resp := ShortenResponse{
		ShortURL: fmt.Sprintf("http://localhost:8081/%s", shortID),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func generateID(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
