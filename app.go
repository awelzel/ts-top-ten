package main

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	http.HandleFunc("/", handle)
	http.HandleFunc("/ts", handleTs)
	http.HandleFunc("/_ah/health", healthCheckHandler)
	log.Print("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type PageVars struct {
	Title    string
	CurDate  string
	NextDate string
	PrevDate string
	L        []ArticlePosition
}

func render_article_positions(w http.ResponseWriter, date time.Time, l []ArticlePosition) (ok bool, err error) {
	tmpl, err := template.ParseFiles("templates/articles.tmpl")
	if err != nil {
		log.Printf("RENDER ERROR: %v", err)
		return false, err
	}

	vars := PageVars{
		"top 10 - archiv",
		date.Format("2006-01-02"),
		date.AddDate(0, 0, 1).Format("2006-01-02"),
		date.AddDate(0, 0, -1).Format("2006-01-02"),
		l}
	err = tmpl.Execute(w, vars)
	if err != nil {
		log.Printf("EXECUTE ERROR: %v", err)
		return false, err
	}
	return true, nil
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	date_str := r.URL.Query().Get("date")
	if date_str == "" {
		date_str = time.Now().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", date_str)
	if err != nil {
		http.Error(w, "date not a date", 400)
		return
	}

	articles, err := FetchArticles(date, date)
	if err != nil {
		log.Printf("Ugh: %s/%v", err, err)
		http.Error(w, "error fetching articles", 500)
		return
	}

	render_article_positions(w, date, articles)
}

func handleTs(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.Error(w, "non cron request", 403)
		return
	}

	resp, err := http.Get("https://www.tagesschau.de")
	if err != nil {
		http.Error(w, "GET failed:", 500)
		return
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "reading http response failed", 500)
		return
	}
	root := soup.HTMLParse(string(b))
	h2s := root.FindAll("h2", "class", "conHeadline")

	var top10 soup.Root
	var found bool

	for _, h2 := range h2s {
		if "Top 10" == h2.Text() {
			top10 = h2
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "top tens not found", 500)
		return
	}

	htmlParent := top10.Pointer.Parent
	sibling := soup.Root{htmlParent, htmlParent.Data, nil}
	linklist := sibling.FindAll("a")
	if len(linklist) != 10 {
		http.Error(w, "no linklist found", 500)
		return
	}

	slice := make([]Article, 10)
	for i, a := range linklist {
		href := strings.TrimSpace(a.Attrs()["href"])
		title := strings.TrimSpace(a.Text())
		x := MakeArticle(href, title)
		slice[i] = x
	}
	if _, err := saveTopTen(slice); err != nil {
		http.Error(w, "unable to store Top 10", 500)
	}
	fmt.Fprintf(w, "<html><body>DONE</body></html>")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "ok")
}
