package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Article struct {
	Link  string
	Title string
	Id    int64
}

func MakeArticle(link string, title string) Article {
	a := Article{Link: link, Title: title}
	return a
}

func (a Article) WebLink() string {
	return fmt.Sprintf("http://www.tagesschau.de%s", a.Link)
}

func (a Article) String() string {
	return fmt.Sprintf("Article{Id:%v, Link:%v}", a.Id, a.Link)
}

type ArticlePosition struct {
	Article      Article
	BestPosition int
	Rating       float32
}

func saveTopTen(articles []Article) (ok bool, err error) {
	db := OpenDB()
	defer db.Close()

	// Work on one article at a time: Better do all at once...
	for i, _ := range articles {
		a := &articles[i]

		// Lookup a DB object or create a new one...
		query_stmt := "SELECT id FROM articles WHERE link = ?"
		rows, err := db.Query(query_stmt, a.Link)
		defer rows.Close()

		if err != nil {
			log.Printf("Query Err: %v", err)
			return false, err
		}
		for rows.Next() {
			if err := rows.Scan(&a.Id); err != nil {
				log.Printf("Scan Err: %v", err)
				return false, err
			}
		}

		// No insert needed...
		if a.Id != 0 {
			continue
		}

		insert_stmt := `INSERT INTO articles (link, title) VALUES (?, ?)`
		result, err := db.Exec(insert_stmt, a.Link, a.Title)
		if err != nil {
			log.Printf("Insert Err: %v %v", err, err.Error())
			return false, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			log.Printf("LastInsertId Err: %v", err)
			return false, err
		}
		a.Id = id
	}

	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Fatalf("Failed to load location: %v", err)
	}
	now := time.Now().In(loc)
	for i, a := range articles {
		position := i + 1
		insert_stmt := `INSERT INTO top_ten_articles
				(article_id, record_time, position) VALUES (?, ?, ?)`
		_, err := db.Exec(insert_stmt, a.Id, now, position)
		if err != nil {
			log.Printf("Insert Err: %v %v", err, err.Error())
			return false, err
		}
	}
	return true, nil
}

func FetchArticles(start_date time.Time, end_date time.Time) (articles []ArticlePosition, err error) {
	db := OpenDB()
	defer db.Close()

	stmt := `SELECT a.id, a.link, a.title,
			min(tt.position) as min_position,
			sum(1.0 / CONVERT(position, DECIMAL(3, 2))) as rating
			FROM articles a JOIN top_ten_articles tt
			ON (a.id = tt.article_id)
		 WHERE date(created_at) >= ? AND date(created_at) <= ?
		 GROUP BY date(a.created_at), a.id
		 ORDER BY rating DESC, a.id
		 `

	rows, err := db.Query(stmt, start_date, end_date)
	if err != nil {
		return nil, err
	}
	articles = make([]ArticlePosition, 0)
	for rows.Next() {
		ap := ArticlePosition{}
		a := &ap.Article

		rows.Scan(&a.Id, &a.Link, &a.Title, &ap.BestPosition, &ap.Rating)
		articles = append(articles, ap)
	}
	return articles, nil

}

func MustGetenv(key string) string {
	if result := os.Getenv(key); result != "" {
		return result
	}
	log.Fatalf("env var %s not set", key)
	return "" // Unreached.
}

func OpenDB() *sql.DB {
	var (
		connString = MustGetenv("CLOUDSQL_CONNECTION_STRING")
		user       = MustGetenv("CLOUDSQL_USER")
		password   = os.Getenv("CLOUDSQL_PASSWORD")
	)
	connString = fmt.Sprintf("%s:%s@%s", user, password, connString)

	db, err := sql.Open("mysql", connString)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	return db
}

func createDatabaseTables() {
	db := OpenDB()
	var err error
	articles_stmt := `CREATE TABLE IF NOT EXISTS articles (
		id INT AUTO_INCREMENT PRIMARY KEY,
		link VARCHAR(255) NOT NULL UNIQUE,
		title VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err = db.Exec(articles_stmt)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	top_ten_stmt := `CREATE TABLE IF NOT EXISTS top_ten_articles  (
		id int AUTO_INCREMENT PRIMARY KEY,
		article_id INT,
		record_time DATETIME NOT NULL,
		position INT NOT NULL,
		FOREIGN KEY (article_id) REFERENCES articles(id),
		UNIQUE (article_id, record_time)
	)`
	_, err = db.Exec(top_ten_stmt)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
}
