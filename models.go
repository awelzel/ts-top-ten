package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type Article struct {
	Link        string
	Title       string
	Id          int64
	Description string
	ImageUrl    string
}

func MakeArticle(link string, title string) Article {
	a := Article{Link: link, Title: title}
	return a
}

func (a Article) WebLink() string {
	return fmt.Sprintf("https://www.tagesschau.de%s", a.Link)
}

func (a Article) String() string {
	return fmt.Sprintf("Article{Id:%v, Link:%v}", a.Id, a.Link)
}

type ArticlePosition struct {
	Article      Article
	BestPosition int
	Rating       float32
}

//XXX Work in batches!
func SaveArticleDetails(a Article, og_description, og_image string) (ok bool, err error) {
	db := OpenDB()
	defer db.Close()
	insert_stmt := `INSERT INTO article_details
			(article_id, og_description, og_image)
			VALUES ($1, $2, $3)`
	_, err = db.Exec(insert_stmt, a.Id, og_description, og_image)
	if err != nil {
		log.Printf("Insert Err: %v %v", err, err.Error())
		return false, err
	}

	return true, nil
}

func SaveTopTen(articles []Article) (ok bool, err error) {
	db := OpenDB()
	defer db.Close()

	// Work on one article at a time: Better do all at once...
	for i, _ := range articles {
		a := &articles[i]

		// Lookup a DB object or create a new one...
		query_stmt := "SELECT id FROM articles WHERE link = $1"
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

		insert_stmt := `INSERT INTO articles ("link", "title")
				VALUES ($1, $2)
				RETURNING id`
		if err := db.QueryRow(insert_stmt, a.Link, a.Title).Scan(&a.Id); err != nil {
			log.Printf("Insert Err: %v %v", err, err.Error())
			return false, err
		}
	}

	now := time.Now().In(time.UTC)
	for i, a := range articles {
		position := i + 1
		insert_stmt := `INSERT INTO top_ten_articles
				(article_id, recorded_at, position) VALUES ($1, $2, $3)`
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
	stmt := `SELECT a.id, a.link, a.title, max(ad.og_description), max(ad.og_image),
			min(tt.position) as min_position,
			sum(1.0 / pow(2, position - 1)) as rating
			FROM articles a JOIN top_ten_articles tt ON (a.id = tt.article_id)
			JOIN article_details ad ON (a.id = ad.article_id)
		 WHERE date(a.created_at) >= $1 AND date(a.created_at) <= $2
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

		rows.Scan(&a.Id, &a.Link, &a.Title, &a.Description, &a.ImageUrl, &ap.BestPosition, &ap.Rating)
		articles = append(articles, ap)
	}
	return articles, nil
}

func FetchArticlesWithoutDetails() (articles []Article, err error) {

	db := OpenDB()
	defer db.Close()
	stmt := `SELECT a.id, a.link, a.title
		 FROM articles a LEFT JOIN article_details ad
		 ON (a.id = ad.article_id) WHERE ad.id IS NULL`
	rows, err := db.Query(stmt)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		a := Article{}
		rows.Scan(&a.Id, &a.Link, &a.Title)
		articles = append(articles, a)
	}
	return articles, nil
}

func MustGetenv(key string) string {
	if result := os.Getenv(key); result != "" {
		return result
	}
	log.Fatalf("environment variable '%s' not set", key)
	return "" // Unreached.
}

func OpenDB() *sql.DB {
	var (
		driver     = MustGetenv("SQL_DRIVER")
		connString = MustGetenv("SQL_CONNECTION_STRING")
	)
	db, err := sql.Open(driver, connString)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	return db
}

func createDatabaseTables() {
	db := OpenDB()
	var err error
	articles_stmt := `CREATE TABLE IF NOT EXISTS articles (
		id SERIAL,
		link VARCHAR(255) NOT NULL UNIQUE,
		title VARCHAR(255),
		created_at TIMESTAMP DEFAULT NOW(),
		PRIMARY KEY(id)
	)`
	_, err = db.Exec(articles_stmt)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	top_ten_stmt := `CREATE TABLE IF NOT EXISTS top_ten_articles  (
		id SERIAL,
		article_id INT NOT NULL,
		recorded_at TIMESTAMP NOT NULL,
		position INT NOT NULL,
		PRIMARY KEY (id),
		FOREIGN KEY (article_id) REFERENCES articles(id),
		UNIQUE (article_id, recorded_at)
	)`
	_, err = db.Exec(top_ten_stmt)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	article_details_stmt := `CREATE TABLE IF NOT EXISTS article_details  (
		id SERIAL,
		article_id INT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		og_description VARCHAR(1024),
		og_image VARCHAR(255),
		PRIMARY KEY (id),
		FOREIGN KEY (article_id) REFERENCES articles(id),
		UNIQUE (article_id)
	)`
	_, err = db.Exec(article_details_stmt)
	if err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
}
