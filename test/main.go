package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kps"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env bulunamadı")
	}
	username := os.Getenv("KPS_USERNAME")
	password := os.Getenv("KPS_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("KPS_USERNAME ve KPS_PASSWORD env değişkenlerini ayarlayın")
	}

	client := kps.New(username, password, nil)

	// Örnek sorgu
	req := kps.QueryRequest{
		TCNo:       "99999999999",
		FirstName:  "JOHN",
		LastName:   "DOE",
		BirthYear:  "1990",
		BirthMonth: "01",
		BirthDay:   "01",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	res, err := client.DoQuery(ctx, req)
	if err != nil {
		log.Fatalf("Hata: %v", err)
	}
	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
}
