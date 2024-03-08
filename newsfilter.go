package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type hnStory struct {
	ID       int    `json:"id"`
	By       string `json:"by"`
	Score    int    `json:"score"`
	Comments int    `json:"descendants"`
	TimeI    int64  `json:"time"`
	Title    string `json:"title"`
	Url      string `json:"url"`
	Type     string `json:"type"`
	Domain   string
	ScoreAvg int
	Time     time.Time
	Hours    int
}

type lrsStory struct {
	ID       string `json:"short_id"`
	TimeS    string `json:"created_at"`
	Title    string `json:"title"`
	Url      string `json:"url"`
	Score    int    `json:"score"`
	Comments int    `json:"comment_count"`
	LrsUrl   string `json:"comments_url"`
	Domain   string
	Time     time.Time
	Hours    int
}

type hnResults struct {
	mainStories     []hnStory
	blockedStories  []hnStory
	lowStories      []hnStory
	permaLowStories []hnStory
	storyIDs        []int
	processedIDs    []int
	urls            []url
}

type url struct {
	url string
	id  int
}

type article struct {
	title  string
	url    string
	domain string
}

var MU = &sync.Mutex{}
var PL = false

func main() {
	var hn hnResults

	homeDir, err := os.UserHomeDir()
	errExit(err, "error: cannot get home dir")
	progDir := homeDir + "/.local/share/newsfilter/"

	client := &http.Client{}
	now := time.Now()

	fmt.Println("getting HN stories...")
	getHnStoryIDs(client, &hn)

	fmt.Println("getting already processed HN IDs...")
	readHnProcessedIDs(&hn, progDir)

	fmt.Println("filtering HN stories...")
	filterHn(&hn, client, now, progDir)

	fmt.Println("getting lobste.rs stories...")
	lrsStories := getLrsStories(client, now)

	fmt.Println("getting already processed lobste.rs IDs...")
	lrsProcessedIDs := readLrsProcessedIDs(progDir)

	fmt.Println("filtering lobste.rs stories...")
	lrsStories = filterLrs(lrsStories, &lrsProcessedIDs)

	//fmt.Println("getting badcyber.com articles...")
	bc := getBadCyberLinks(client, progDir)

	fmt.Println("logging all stories...")
	logHnStories(&hn, progDir)
	logLrsStories(lrsStories, progDir)
	//logBcArticles(bc, progDir)

	fmt.Println("reading history of HN URLs...")
	readHnUrls(&hn, progDir)

	fmt.Println("preparing final html file...")
	prepareHtml(&hn, &lrsStories, &bc, progDir, now)

	fmt.Println("\nHN stats")
	fmt.Printf("fetched stories: %d\n"+
		"processed stories: %d\n"+
		"blocked stories: %d\n"+
		"low score stories: %d\n"+
		"permanently low score stories: %d\n"+
		"main stories: %d\n",
		len(hn.storyIDs),
		len(hn.processedIDs),
		len(hn.blockedStories),
		len(hn.lowStories),
		len(hn.permaLowStories),
		len(hn.mainStories))

	fmt.Println("\nlobste.rs stats")
	fmt.Printf("processed stories: %d\n"+
		"main stories: %d\n",
		len(lrsProcessedIDs),
		len(lrsStories))

	/*
	fmt.Println("\nbadcyber.com stats")
	var bcCount int
	for _, articles := range bc {
		bcCount += len(articles)
	}
	fmt.Printf("catch ups: %d\n"+
		"articles: %d\n\n",
		len(bc),
		bcCount)
	*/

	dt := fmt.Sprintf("%d-%.2d-%.2d_%.2d%.2d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())
	outFile := "news_" + dt + ".html"
	fmt.Println(progDir + outFile)

	clearTmp(progDir)
}

func readBlockedDomains(progDir string) []string {
	var blockedDomains []string

	f, err := os.Open(progDir + "blocked.domains")
	errExit(err, "error: cannot read file")
	defer f.Close()

	input := bufio.NewScanner(f)
	for input.Scan() {
		blockedDomains = append(blockedDomains, input.Text())
	}
	sort.Strings(blockedDomains)

	return blockedDomains
}

func readBlockedKeywords(progDir string) []string {
	var blockedKeywords []string

	f, err := os.Open(progDir + "blocked.keywords")
	errExit(err, "error: cannot read file")
	defer f.Close()

	input := bufio.NewScanner(f)
	for input.Scan() {
		blockedKeywords = append(blockedKeywords, input.Text())
	}
	sort.Strings(blockedKeywords)

	return blockedKeywords
}

func readHnProcessedIDs(hn *hnResults, progDir string) {
	fd, err := os.Open(progDir + "hn_processed_ids")
	defer fd.Close()
	if err != nil {
		return
	}

	input := bufio.NewScanner(fd)
	for input.Scan() {
		id, err := strconv.Atoi(input.Text())
		errExit(err, "error: cannot read processed ID")
		hn.processedIDs = append(hn.processedIDs, id)
	}
	sort.Ints(hn.processedIDs)
}

func readHnUrls(hn *hnResults, progDir string) {
	files := []string{"hn_main.tsv", "hn_permalow.tsv",
		"hn_blocked.tsv", "hn_low.tsv.tmp"}

	for _, f := range files {
		fd, err := os.Open(progDir + f)
		defer fd.Close()
		if err != nil {
			return
		}

		input := bufio.NewScanner(fd)
		for input.Scan() {
			s := strings.Split(input.Text(), "\t")
			u := s[8]
			i, err := strconv.Atoi(s[2])
			errExit(err, "error: cannot read story ID: "+s[2])

			hn.urls = append(hn.urls, url{url: u, id: i})
		}
	}

	sort.Slice(hn.urls, func(i, j int) bool {
		return hn.urls[i].url <= hn.urls[j].url
	})
}

func readLrsProcessedIDs(progDir string) []string {
	var processedIDs []string

	fd, err := os.Open(progDir + "lrs_processed_ids")
	defer fd.Close()
	if err != nil {
		return processedIDs
	}

	input := bufio.NewScanner(fd)
	for input.Scan() {
		processedIDs = append(processedIDs, input.Text())
	}
	sort.Strings(processedIDs)

	return processedIDs
}

func readBcLinks(progDir string) []string {
	var bcLinks []string

	fd, err := os.Open(progDir + "bc_links.txt")
	defer fd.Close()
	if err != nil {
		return bcLinks
	}

	input := bufio.NewScanner(fd)
	for input.Scan() {
		link := strings.Split(input.Text(), "\t")[0]
		bcLinks = append(bcLinks, link)
	}
	sort.Strings(bcLinks)

	return bcLinks
}

func strExists(s []string, el string) bool {
	i := sort.SearchStrings(s, el)
	if i >= len(s) {
		return false
	}
	return s[i] == el
}

func urlExists(hn *hnResults, url string) (bool, int) {
	idx := sort.Search(len(hn.urls), func(i int) bool {
		return string(hn.urls[i].url) >= url
	})

	// todo: this line panics
	hnUrl := strings.TrimSuffix(hn.urls[idx].url, "/")
	url = strings.TrimSuffix(url, "/")

	if hnUrl == url {
		return true, idx
	} else {
		return false, 0
	}
}

func intExists(s []int, el int) bool {
	i := sort.SearchInts(s, el)
	if i >= len(s) {
		return false
	}
	return s[i] == el
}

func keywordFound(keywords []string, title string) bool {
	for _, line := range keywords {
		words := strings.Split(line, "\t")

		if words[0] == "" || strings.HasPrefix(words[0], "#") {
			continue
		}

		if strings.Contains(title, words[0]) {
			if blockOverride(words, title) {
				continue
			} else {
				return true
			}
		}
	}
	return false
}

func blockOverride(words []string, title string) bool {
	for _, word := range words[1:] {
		if word[0] != '!' {
			errExit(errors.New(""), "incorrect keyword: "+word)
		}
		word = strings.TrimPrefix(word, "!")

		if strings.Contains(title, word) {
			return true
		} else {
			continue
		}
	}
	return false
}

func blockDomain(domains []string, domain string) bool {
	for _, blockedDomain := range domains {
		switch {
		case blockedDomain == "":
			continue
		case strings.HasPrefix(blockedDomain, "*"):
			blockedDomain = strings.TrimPrefix(blockedDomain, "*")
			if strings.HasSuffix(domain, blockedDomain) {
				return true
			}
		case domain == blockedDomain:
			return true
		}
	}
	return false
}

func getHnStoryIDs(client *http.Client, hn *hnResults) {
	var topIDs, bestIDs []int
	urlTop := "https://hacker-news.firebaseio.com/v0/topstories.json"
	urlBest := "https://hacker-news.firebaseio.com/v0/beststories.json"

	req, err := http.NewRequest("GET", urlTop, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	err = json.Unmarshal(body, &topIDs)
	errExit(err, "error: cannot parse json")

	req, err = http.NewRequest("GET", urlBest, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err = client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	err = json.Unmarshal(body, &bestIDs)
	errExit(err, "error: cannot parse json")

	hn.storyIDs = append(topIDs, bestIDs...)
	sort.Ints(hn.storyIDs)
	hn.storyIDs = uniqueInts(hn.storyIDs)
}

func getLrsStories(client *http.Client, now time.Time) []lrsStory {

	var storiesHot, storiesNew []lrsStory
	urlHot := "https://lobste.rs/hottest.json"
	urlNew := "https://lobste.rs/newest.json"

	req, err := http.NewRequest("GET", urlHot, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	err = json.Unmarshal(body, &storiesHot)
	errExit(err, "error: cannot parse json")

	req, err = http.NewRequest("GET", urlNew, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err = client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	err = json.Unmarshal(body, &storiesNew)
	errExit(err, "error: cannot parse json")

	stories := append(storiesHot, storiesNew...)

	for i, story := range stories {
		if story.Url == "" {
			(&stories[i]).Url = story.LrsUrl
			(&stories[i]).Domain = "lobste.rs"
		} else {
			(&stories[i]).Domain = urlToDomain(story.Url)
		}

		layout := "2006-01-02T15:04:05.999999999Z07:00"
		t, err := time.Parse(layout, story.TimeS)
		errExit(err, "error: cannot parse time")
		local, _ := time.LoadLocation("Local")

		(&stories[i]).Time = t.In(local)
		(&stories[i]).Hours = int(now.Sub(t.In(local)).Hours())
	}

	return stories
}

func uniqueInts(ints []int) []int {
	j := 1
	for i := 1; i < len(ints); i++ {
		if ints[i] != ints[i-1] {
			ints[j] = ints[i]
			j++
		}
	}
	return ints[:j]
}

func urlToDomain(url string) string {
	urlSplit := strings.Split(url, "/")
	domain := urlSplit[2]

	if (domain == "github.com" || domain == "gitlab.com") &&
		len(urlSplit) > 3 {

		domain += "/" + urlSplit[3]
	}

	prefixes := []string{"git.", "www.", "engineering."}
	for _, p := range prefixes {
		dots := len(strings.Split(domain, "."))
		if dots > 1 && strings.HasPrefix(domain, p) {
			domain = strings.TrimPrefix(domain, p)
		}
	}

	return domain
}

func getStory(id int, client *http.Client, now time.Time) hnStory {
	var story hnStory

	url := "https://hacker-news.firebaseio.com/v0/item/" +
		strconv.Itoa(id) + ".json"

	req, err := http.NewRequest("GET", url, nil)
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	err = json.Unmarshal(body, &story)

	story.By = strings.Replace(story.By, "\t", " ", -1)
	story.Title = strings.Replace(story.Title, "\t", " ", -1)

	if story.Url == "" {
		story.Url = "https://news.ycombinator.com/item?id=" +
			strconv.Itoa(id)
	}
	story.Domain = urlToDomain(story.Url)
	story.Time = time.Unix(story.TimeI, 0)
	story.Hours = int(now.Sub(story.Time).Hours())
	if story.Hours == 0 {
		story.Hours = 1
	}
	story.ScoreAvg = story.Score / story.Hours

	return story
}

func filterHn(hn *hnResults, client *http.Client, now time.Time,
	progDir string) {

	wg := sync.WaitGroup{}

	blockedDomains := readBlockedDomains(progDir)
	blockedKeywords := readBlockedKeywords(progDir)

	for _, id := range hn.storyIDs {

		if intExists(hn.processedIDs, id) {
			continue
		}

		wg.Add(1)

		go func(id int) {
			story := getStory(id, client, now)
			MU.Lock()
			classifyStory(story,
				blockedDomains, blockedKeywords, hn)
			MU.Unlock()
			wg.Done()
		}(id)
	}

	wg.Wait()

	sort.Slice(hn.mainStories, func(i, j int) bool {
		return hn.mainStories[i].ID <= hn.mainStories[j].ID
	})
}

func classifyStory(story hnStory, blockedDomains, blockedKeywords []string,
	hn *hnResults) {

	switch {
	case story.Type != "story":
		hn.blockedStories = append(hn.blockedStories, story)

	case blockDomain(blockedDomains, story.Domain):
		hn.blockedStories = append(hn.blockedStories, story)

	case keywordFound(blockedKeywords, story.Title):
		hn.blockedStories = append(hn.blockedStories, story)

	case story.Hours > 72 && story.Score >= 100:
		hn.mainStories = append(hn.mainStories, story)

	case story.Comments >= 40:
		hn.mainStories = append(hn.mainStories, story)

	case story.Hours > 72 && story.Score < 100:
		hn.permaLowStories = append(hn.permaLowStories, story)

	case story.Hours > 36 && story.Score < 50:
		hn.permaLowStories = append(hn.permaLowStories, story)

	case story.Hours > 24 && story.Score < 20:
		hn.permaLowStories = append(hn.permaLowStories, story)

	case story.Hours > 12 && story.Score < 10:
		hn.permaLowStories = append(hn.permaLowStories, story)

	case story.Score < 100 && story.ScoreAvg < 20:
		hn.lowStories = append(hn.lowStories, story)

	default:
		hn.mainStories = append(hn.mainStories, story)
	}
}

func filterLrs(lrsStories []lrsStory, lrsProcessedIDs *[]string) []lrsStory {
	var result []lrsStory

	for _, story := range lrsStories {
		if strExists(*lrsProcessedIDs, story.ID) {
			continue
		}
		if story.Score > 20 || story.Comments > 5 {
			result = append(result, story)
			*lrsProcessedIDs = append(*lrsProcessedIDs, story.ID)
			// expensive; fix if processing slows down
			sort.Strings(*lrsProcessedIDs)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Hours >= result[j].Hours
	})

	return result
}

func logHnStories(hn *hnResults, progDir string) {
	storiesToFile(progDir, "hn_main.tsv", hn.mainStories, true)
	storiesToFile(progDir, "hn_blocked.tsv", hn.blockedStories, true)
	storiesToFile(progDir, "hn_permalow.tsv", hn.permaLowStories, true)
	storiesToFile(progDir, "hn_low.tsv.tmp", hn.lowStories, false)
}

func storiesToFile(progDir, file string, stories []hnStory, logID bool) {
	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY

	fd, err := os.OpenFile(progDir+file, fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fdIDs, err := os.OpenFile(progDir+"hn_processed_ids", fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fdIDs.Close()

	for _, story := range stories {
		fmt.Fprintln(fd, logHnLine(story))
		if logID {
			fmt.Fprintln(fdIDs, story.ID)
		}
	}
}

func logLrsStories(lrsStories []lrsStory, progDir string) {
	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY

	fd, err := os.OpenFile(progDir+"lrs_main.tsv", fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fdIDs, err := os.OpenFile(progDir+"lrs_processed_ids", fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fdIDs.Close()

	for _, story := range lrsStories {
		fmt.Fprintln(fd, logLrsLine(story))
		fmt.Fprintln(fdIDs, story.ID)
	}
}

func logBcArticles(bc map[string][]article, progDir string) {
	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY

	fd, err := os.OpenFile(progDir+"bc_links.txt", fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	for link, _ := range bc {
		fmt.Fprintln(fd, link)
	}
}

func logHnLine(story hnStory) string {
	return fmt.Sprintf("%s\t"+
		"%2.2d:%2.2d\t"+
		"%d\t"+
		"%d\t"+
		"%d\t"+
		"%d\t"+
		"%s\t"+
		"%s\t"+
		"%s",
		story.Time.Format("2006-01-02"),
		story.Time.Hour(), story.Time.Minute(),
		story.ID,
		story.Hours,
		story.Score,
		story.ScoreAvg,
		story.By,
		story.Title,
		story.Url,
	)
}

func logLrsLine(story lrsStory) string {
	return fmt.Sprintf("%s\t"+
		"%s\t"+
		"%s\t"+
		"%s",
		story.TimeS,
		story.ID,
		story.Title,
		story.Url,
	)
}

func getBadCyberLinks(client *http.Client, progDir string) map[string][]article {
	bc := make(map[string][]article)
	return bc

	catchUpLinks := getCatchUpLinks(client, progDir)
	for _, link := range catchUpLinks {
		title, articles := getCatchUpArticles(link, client)
		bc[link+"\t"+title] = articles
	}

	return bc
}

func getCatchUpLinks(client *http.Client, progDir string) []string {
	var catchUpLinks []string

	url := "https://badcyber.com/feed/"
	if PL {
		url = "https://zaufanatrzeciastrona.pl/feed/"
	}

	req, err := http.NewRequest("GET", url, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	bcLinks := readBcLinks(progDir)

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		link := getCatchUpLink(line)
		if link != "" && !strExists(bcLinks, link) {
			catchUpLinks = append(catchUpLinks, link)
		}
	}

	return catchUpLinks
}

func getCatchUpArticles(url string, client *http.Client) (string, []article) {
	var articles []article

	req, err := http.NewRequest("GET", url, nil)
	errExit(err, "error: cannot prepare a request")
	req.Close = true
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	title, rawArticles := getRawArticles(string(body))
	for _, rawA := range rawArticles {
		if !strings.Contains(rawA, ">") ||
			!strings.Contains(rawA, "http") {

			continue
		}

		var a article
		rawA = strings.Replace(rawA, "<strong>", "", -1)
		rawA = strings.Replace(rawA, "</strong>", "", -1)
		split := strings.Split(rawA, ">")
		a.url = strings.Split(split[0], "\" ")[0]
		a.title = strings.Split(split[1], "</a")[0]
		a.domain = urlToDomain(a.url)

		articles = append(articles, a)
	}

	return title, articles
}

func getRawArticles(htmlBody string) (string, []string) {
	var title string
	var rawArticles []string

	lines := strings.Split(htmlBody, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "<h1 class=\"entry-title\">") ||
			strings.Contains(line, "<h1>") {

			title = strings.Split(line, ">")[1]
			title = strings.Split(title, "<")[0]
		}

		if strings.HasPrefix(line, "<li>") &&
			strings.HasSuffix(line, "</a></li>") &&
			!strings.Contains(line, "https://badcyber.com") {

			split := strings.Split(line, "href=\"")
			for _, item := range split {
				rawArticles = append(rawArticles, item)
			}
		}
	}

	if len(rawArticles) == 0 {
		fmt.Println("couldn't get badcyber.com articles")
	}

	return title, rawArticles
}

func getCatchUpLink(s string) string {
	var link string
	keyWord := "it-security-weekend-catch-up"
	if PL {
		keyWord = "weekendowa-lektura-odcinek"
	}

	if strings.Contains(s, "<link>") && strings.Contains(s, keyWord) {
		link = strings.Split(s, "<link>")[1]
		link = strings.Split(link, "</link>")[0]
	}

	return link
}

func prepareHtml(hn *hnResults, lrsStories *[]lrsStory,
	bc *map[string][]article, progDir string, now time.Time) {

	dt := fmt.Sprintf("%d-%.2d-%.2d_%.2d%.2d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())
	file := "news_" + dt + ".html"

	fdOpts := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	fd, err := os.OpenFile(progDir+file, fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fmt.Fprintln(fd, htmlHeader)

	if len(hn.mainStories) > 0 {
		fmt.Fprintln(fd, "* hacker news\n")
	}

	for _, story := range hn.mainStories {
		printHnStory(fd, story)
	}

	if len(*lrsStories) > 0 {
		fmt.Fprintln(fd, "\n* lobste.rs\n")
	}
	for _, story := range *lrsStories {
		printLrsStory(fd, story, hn)
	}

	if len(*bc) > 0 {
		fmt.Fprintln(fd, "\n* badcyber.com\n")
	}
	for link, articles := range *bc {
		linkSplit := strings.Split(link, "\t")
		link, title := linkSplit[0], linkSplit[1]
		header := fmt.Sprintf("%s (<a href='%s'>link</a>)\n",
			title, link)
		fmt.Fprintln(fd, header)
		for i, a := range articles {
			newLine := "\n"
			if i < len(articles)-1 && isPrevArticle(articles[i+1]) {
				newLine = ""
			}
			printBcArticle(fd, a, hn, newLine)
		}
		fmt.Fprintln(fd, "\n")
	}

	fmt.Fprintln(fd, htmlFooter)
}

func isPrevArticle(a article) bool {
	t := a.title
	switch {
	case strings.Contains(t, "PDF") && len(t) <= 8:
		return true
	case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
		return true
	case strings.HasPrefix(t, "komunikat") && len(t) <= 32:
		return true
	case strings.HasPrefix(t, "szczegóły") && len(t) <= 32:
		return true
	case strings.HasPrefix(t, "more info") && len(t) <= 32:
		return true
	case len(t) <= 10:
		return true
	}
	return false
}

func printHnStory(fd *os.File, story hnStory) {
	hnItemUrl := "https://news.ycombinator.com/item?id="
	hnUrl := hnItemUrl + strconv.Itoa(story.ID)

	printString := fmt.Sprintf("<a href='%s'>%s</a>\n"+
		"%dh ago, %d points, <a href='%s'>%d comments</a> "+
		"(<a href='https://news.ycombinator.com/from?site=%s'>"+
		"%s</a>)\n",
		story.Url,
		story.Title,
		story.Hours,
		story.Score,
		hnUrl,
		story.Comments,
		story.Domain,
		story.Domain,
	)

	fmt.Fprintln(fd, printString)
}

func printLrsStory(fd *os.File, story lrsStory, hn *hnResults) {
	hnItemUrl := "https://news.ycombinator.com/item?id="
	hnLink := "-"

	hnExists, idx := urlExists(hn, story.Url)
	if hnExists {
		hnUrl := hnItemUrl + strconv.Itoa(hn.urls[idx].id)
		hnLink = fmt.Sprintf("<a href='%s'>hn</a>", hnUrl)
	}

	printString := fmt.Sprintf("<a href='%s'>%s</a>\n"+
		"%dh ago, %d points, <a href='%s'>%d comments</a> "+
		"(<a href='https://lobste.rs/domain/%s'>%s</a>) (%s)\n",
		story.Url,
		story.Title,
		story.Hours,
		story.Score,
		story.LrsUrl,
		story.Comments,
		story.Domain,
		story.Domain,
		hnLink,
	)

	fmt.Fprintln(fd, printString)
}

func printBcArticle(fd *os.File, a article, hn *hnResults, newLine string) {
	hnItemUrl := "https://news.ycombinator.com/item?id="
	hnLink := "-"

	hnExists, idx := urlExists(hn, a.url)
	if hnExists {
		hnUrl := hnItemUrl + strconv.Itoa(hn.urls[idx].id)
		hnLink = fmt.Sprintf("<a href='%s'>hn</a>", hnUrl)
	}
	printString := fmt.Sprintf("<a href='%s'>%s</a>\n(%s) (%s)\n",
		a.url, a.title, hnLink, a.domain)

	fmt.Fprintf(fd, "%s%s", printString, newLine)
}

func clearTmp(progDir string) {
	tmpFile := progDir + "hn_low.tsv.tmp"
	info, _ := os.Stat(tmpFile)
	if info.Size() > 8*1024*1024 {
		os.Remove(tmpFile)
	}
}

func errExit(err error, msg string) {
	if err != nil {
		log.Println("\n * " + msg)
		log.Fatal(err)
	}
}

var htmlHeader string = `<!DOCTYPE html>
<html>
<head>
	<title>news</title>
	<meta charset="UTF-8">
</head>

<style>
	html, pre {	max-width: 750px;
			margin: 0 auto;
			line-height: 1.2;
			color: #bbb;
			background-color: #000;
			-webkit-font-smoothing: none;
			-webkit-text-stroke: 0.3px;
	}
	a {		color: #009900;
			background-color: transparent;
			text-decoration: underline;
	}
</style>

<body><pre>
`

var htmlFooter string = `</pre></body>
</html>`
