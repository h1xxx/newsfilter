package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
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
}

type hnResults struct {
	mainStories    []hnStory
	blockedStories []hnStory
	lowStories     []hnStory
	vLowStories    []hnStory
	storyIDs       []int
	processedIDs   []int
	urls           []url
}

type url struct {
	url string
	id  int
}

func main() {
	var hn hnResults

	homeDir, err := os.UserHomeDir()
	errExit(err, "error: cannot get home dir")
	progDir := homeDir + "/.config/newsfilter/"

	client := &http.Client{}
	now := time.Now()

	getHnStoryIDs(client, &hn)
	readHnProcessedIDs(&hn, progDir)
	filterHn(&hn, client, now)

	lrsStories := getLrsStories(client)
	lrsProcessedIDs := readLrsProcessedIDs(progDir)
	lrsStories = filterLrs(lrsStories, &lrsProcessedIDs)

	logHnStories(&hn, progDir)
	logLrsStories(lrsStories, progDir)

	readHnUrls(&hn, progDir)
	fmt.Println(len(hn.urls))
	prepareHtml(&hn, &lrsStories, progDir, now)

	fmt.Printf("all: %d\n"+
		"processed: %d\n"+
		"blocked: %d\n"+
		"low: %d\n"+
		"very low: %d\n"+
		"main: %d\n",
		len(hn.storyIDs),
		len(hn.processedIDs),
		len(hn.blockedStories),
		len(hn.lowStories),
		len(hn.vLowStories),
		len(hn.mainStories))

	fmt.Println(len(lrsStories), len(lrsProcessedIDs))
}

func readBlockedDomains() []string {
	var blockedDomains []string

	f, err := os.Open("blocked.domains")
	errExit(err, "error: cannot read file")
	defer f.Close()

	input := bufio.NewScanner(f)
	for input.Scan() {
		blockedDomains = append(blockedDomains, input.Text())
	}
	sort.Strings(blockedDomains)

	return blockedDomains
}

func readBlockedKeywords() []string {
	var blockedKeywords []string

	f, err := os.Open("blocked.keywords")
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
	files := []string{"hn_main.tsv", "hn_vlow.tsv",
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
			u := s[7]
			i, err := strconv.Atoi(s[1])
			errExit(err, "error: cannot read story ID")

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

	if hn.urls[idx].url == url {
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
	for _, k := range keywords {
		switch {
		case k == "":
			continue
		case strings.Contains(title, k):
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
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &topIDs)
	errExit(err, "error: cannot parse json")

	req, err = http.NewRequest("GET", urlBest, nil)
	errExit(err, "error: cannot prepare a request")
	resp, err = client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err = ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &bestIDs)
	errExit(err, "error: cannot parse json")

	hn.storyIDs = append(topIDs, bestIDs...)
	sort.Ints(hn.storyIDs)
	hn.storyIDs = uniqueInts(hn.storyIDs)
}

func getLrsStories(client *http.Client) []lrsStory {

	var storiesHot, storiesNew []lrsStory
	urlHot := "https://lobste.rs/hottest.json"
	urlNew := "https://lobste.rs/newest.json"

	req, err := http.NewRequest("GET", urlHot, nil)
	errExit(err, "error: cannot prepare a request")
	resp, err := client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &storiesHot)
	errExit(err, "error: cannot parse json")

	req, err = http.NewRequest("GET", urlNew, nil)
	errExit(err, "error: cannot prepare a request")
	resp, err = client.Do(req)
	errExit(err, "error: cannot make a request")

	body, err = ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &storiesNew)
	errExit(err, "error: cannot parse json")

	return append(storiesHot, storiesNew...)
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

func getStory(id int, client *http.Client, now time.Time) hnStory {
	var story hnStory

	url := "https://hacker-news.firebaseio.com/v0/item/" +
		strconv.Itoa(id) + ".json"

	req, err := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &story)

	story.By = strings.Replace(story.By, "\t", " ", -1)
	story.Title = strings.Replace(story.Title, "\t", " ", -1)

	if story.Url == "" {
		story.Url = "https://news.ycombinator.com/item?id=" +
			strconv.Itoa(id)
	}
	story.Domain = strings.Split(story.Url, "/")[2]
	story.Time = time.Unix(story.TimeI, 0)
	story.Hours = int(now.Sub(story.Time).Hours())
	if story.Hours == 0 {
		story.Hours = 1
	}
	story.ScoreAvg = story.Score / story.Hours

	return story
}

func filterHn(hn *hnResults, client *http.Client, now time.Time) {
	blockedDomains := readBlockedDomains()
	blockedKeywords := readBlockedKeywords()

	for _, id := range hn.storyIDs {

		if intExists(hn.processedIDs, id) {
			continue
		}

		story := getStory(id, client, now)
		classifyStory(story, blockedDomains, blockedKeywords, hn)
	}
}

func classifyStory(story hnStory, blockedDomains, blockedKeywords []string,
	hn *hnResults) {

	switch {
	case story.Type != "story":
		hn.blockedStories = append(hn.blockedStories, story)

	case strExists(blockedDomains, story.Domain):
		hn.blockedStories = append(hn.blockedStories, story)

	case keywordFound(blockedKeywords, story.Title):
		hn.blockedStories = append(hn.blockedStories, story)

	case story.Hours > 72 && story.Score >= 100:
		hn.mainStories = append(hn.mainStories, story)

	case story.Hours > 72 && story.Score < 100:
		hn.vLowStories = append(hn.vLowStories, story)

	case story.Hours > 36 && story.Score < 50:
		hn.vLowStories = append(hn.vLowStories, story)

	case story.Hours > 24 && story.Score < 20:
		hn.vLowStories = append(hn.vLowStories, story)

	case story.Hours > 12 && story.Score < 10:
		hn.vLowStories = append(hn.vLowStories, story)

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
		if story.Score > 10 || story.Comments > 5 {
			result = append(result, story)
			*lrsProcessedIDs = append(*lrsProcessedIDs, story.ID)
			// expensive; fix if processing slows down
			sort.Strings(*lrsProcessedIDs)
		}
	}
	return result
}

func logHnStories(hn *hnResults, progDir string) {
	storiesToFile(progDir, "hn_main.tsv", hn.mainStories, true)
	storiesToFile(progDir, "hn_blocked.tsv", hn.blockedStories, true)
	storiesToFile(progDir, "hn_vlow.tsv", hn.vLowStories, true)
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

func logHnLine(story hnStory) string {
	return fmt.Sprintf("%s\t"+
		"%d\t"+
		"%d\t"+
		"%d\t"+
		"%d\t"+
		"%s\t"+
		"%s\t"+
		"%s",
		story.Time.Format("2006-01-02"),
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

func prepareHtml(hn *hnResults, lrsStories *[]lrsStory, progDir string,
	now time.Time) {

	dt := fmt.Sprintf("%d-%.2d-%.2d_%.2d%.2d",
		now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())
	file := "news_" + dt + ".html"
	hnItemUrl := "https://news.ycombinator.com/item?id="

	fdOpts := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	fd, err := os.OpenFile(progDir+file, fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fmt.Fprintln(fd, htmlHeader)

	for _, story := range hn.mainStories {
		fmt.Fprintf(fd, "%s\n", story.Title)
		fmt.Fprintf(fd, "<a href='%s'>link</a>", story.Url)
		hnUrl := hnItemUrl + strconv.Itoa(story.ID)
		fmt.Fprintf(fd, " <a href='%s'>hn</a>", hnUrl)
		fmt.Fprintln(fd, "\n")
	}

	for _, story := range *lrsStories {
		hnExists, idx := urlExists(hn, story.Url)

		fmt.Fprintf(fd, "%s\n", story.Title)
		fmt.Fprintf(fd, "<a href='%s'>link</a>", story.Url)
		fmt.Fprintf(fd, " <a href='%s'>lrs</a>", story.LrsUrl)
		if hnExists {
			hnUrl := hnItemUrl + strconv.Itoa(hn.urls[idx].id)
			fmt.Fprintf(fd, " <a href='%s'>hn</a>", hnUrl)
		} else {
			fmt.Fprintf(fd, " -")
		}
		fmt.Fprintln(fd, "\n")
	}

	fmt.Fprintln(fd, htmlFooter)
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
