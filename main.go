package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	connStr := "postgres://postgres:123456@localhost:5432/urlshortener?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
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

	fmt.Printf("✅ Tạo thành công!\nURL gốc: %s\nURL ngắn: http://localhost:8080/%s\n", longURL, shortID)
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
