package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type PageData struct {
	Header string
}

type Link struct {
	ID int
	URL string
	Short string
	Date string
}

func generateRandomAlphaID(n int) (string, error) {

	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := make([]byte, n)

	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}

	return string(bytes), nil
}

func adminAuth(w http.ResponseWriter, r *http.Request) bool {

	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")

	user, pass, ok := r.BasicAuth()

	if !ok || user != adminUser || pass != adminPass {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	return true

}

func adminHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {

	if !adminAuth(w, r) {
		return
	}

	rows, err := db.Query("SELECT `index`, url, short, date FROM links")

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		var t time.Time
	
		if err := rows.Scan(&l.ID, &l.URL, &l.Short, &t); err != nil {
			http.Error(w, "Error scanning database", http.StatusInternalServerError)
			return
		}
	
		l.Date = t.Format("2006-01-02 15:04:05")
		links = append(links, l)
	}

	adminTmpl, err := template.ParseFiles("./assets/admin/admin.html")

	if err != nil {
		http.Error(w, "Error loading admin page", http.StatusInternalServerError)
		return
	}

	data := struct {
		Links []Link
	}{
		Links: links,
	}

	adminTmpl.Execute(w, data)

}


func adminDeleteHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !adminAuth(w, r) {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	idStr := r.FormValue("id")

	if idStr == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)

	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM links WHERE `index` = ?", id)
	if err != nil {
		http.Error(w, "Database deletion error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)

}

func main() {

	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	hostUri := os.Getenv("HOST_URI")

	if dbUser == "" || dbPassword == "" || dbHost == "" || dbPort == "" || dbName == "" {
		log.Fatal("Database configuration is missing in environment variables")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("mysql", dsn)

	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	defer db.Close()

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("Waiting for database... (%d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database after %d attempts: %v", maxRetries, err)
	}

	log.Println("Connected to MySQL successfully.")

	tmpl, err := template.ParseGlob("./assets/*.html")

	if err != nil {
		log.Fatalf("Template parsing error: %v", err)
	}

	router := http.NewServeMux()
	router.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("./assets/css"))))
	router.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./assets/js"))))
	router.Handle("/imgs/", http.StripPrefix("/imgs/", http.FileServer(http.Dir("./assets/imgs"))))

	router.HandleFunc("/shortify", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		originalURL := r.FormValue("url")

		if originalURL == "" {
			http.Error(w, "URL is required", http.StatusBadRequest)
			return
		}

		var shortURL string
		err := db.QueryRow("SELECT short FROM links WHERE url = ?", originalURL).Scan(&shortURL)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"shortURL": shortURL})
			return
		}

		id, err := generateRandomAlphaID(3)

		if err != nil {
			http.Error(w, "Error generating ID", http.StatusInternalServerError)
			return
		}

		host := hostUri
		if host == "" {
			host = r.Host
		}

		shortURL = fmt.Sprintf("http://%s/%s", host, id)

		_, err = db.Exec("INSERT INTO links (url, short) VALUES (?, ?)", originalURL, shortURL)
		if err != nil {
			http.Error(w, "Database insert error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"shortURL": shortURL})

	})

	router.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		adminHandler(w, r, db)
	})

	router.HandleFunc("/admin/delete", func(w http.ResponseWriter, r *http.Request) {
		adminDeleteHandler(w, r, db)
	})

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/" {
			host := hostUri

			if host == "" {
				host = r.Host
			}

			fullShortURL := fmt.Sprintf("http://%s%s", host, r.URL.Path)

			var originalURL string

			err := db.QueryRow("SELECT url FROM links WHERE short = ?", fullShortURL).Scan(&originalURL)

			if err != nil {
				if err == sql.ErrNoRows {
					tmpl.ExecuteTemplate(w, "404.html", PageData{Header: "Page cannot be found"})
				} else {
					http.Error(w, "Database error", http.StatusInternalServerError)
				}
				return
			}

			http.Redirect(w, r, originalURL, http.StatusFound)
			return

		}

		data := PageData{
			Header: "Shortify in GO",
		}

		tmpl.ExecuteTemplate(w, "index.html", data)

	})

	srv := http.Server{
		Addr: ":80",
		Handler: router,
	}

	fmt.Println("Starting website at :80")

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server error: %v", err)
	}

}
