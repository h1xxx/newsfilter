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

type results struct {
	mainStories    []hnStory
	blockedStories []hnStory
	lowStories     []hnStory
	vLowStories    []hnStory
	storyIDs       []int
	processedIDs   []int
}

func main() {
	var r results

	blockedDomains := readBlockedDomains()
	blockedKeywords := readBlockedKeywords()

	client := &http.Client{}
	now := time.Now()

	getStoryIDs(client, &r)
	readProcessedIDs(&r)

	count := 0
	for _, id := range r.storyIDs {

		if intExists(r.processedIDs, id) {
			continue
		}

		story := getStory(id, client, now)
		classifyStory(story, blockedDomains, blockedKeywords, &r)

		count++
		if count > 20 {
			break
		}
	}

	logStories(&r)
	prepareHtml(&r)

	fmt.Printf("all: %d\n"+
		"processed: %d\n"+
		"blocked: %d\n"+
		"low: %d\n"+
		"very low: %d\n"+
		"main: %d\n",
		len(r.storyIDs),
		len(r.processedIDs),
		len(r.blockedStories),
		len(r.lowStories),
		len(r.vLowStories),
		len(r.mainStories))
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

func readProcessedIDs(r *results) {
	fd, err := os.Open("/home/x/.config/newsfilter/processed_ids")
	defer fd.Close()
	if err != nil {
		return
	}

	input := bufio.NewScanner(fd)
	for input.Scan() {
		id, err := strconv.Atoi(input.Text())
		errExit(err, "error: cannot read processed ID")
		r.processedIDs = append(r.processedIDs, id)
	}
	sort.Ints(r.processedIDs)
}

func strExists(s []string, el string) bool {
	i := sort.SearchStrings(s, el)
	if i >= len(s) {
		return false
	}
	return s[i] == el
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

func getStoryIDs(client *http.Client, r *results) {
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

	r.storyIDs = append(topIDs, bestIDs...)
	sort.Ints(r.storyIDs)
	r.storyIDs = uniqueInts(r.storyIDs)
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

func classifyStory(story hnStory, blockedDomains, blockedKeywords []string,
	r *results) {

	switch {
	case story.Type != "story":
		r.blockedStories = append(r.blockedStories, story)

	case strExists(blockedDomains, story.Domain):
		r.blockedStories = append(r.blockedStories, story)

	case keywordFound(blockedKeywords, story.Title):
		r.blockedStories = append(r.blockedStories, story)

	case story.Hours > 72 && story.Score >= 100:
		r.mainStories = append(r.mainStories, story)

	case story.Hours > 72 && story.Score < 100:
		r.vLowStories = append(r.vLowStories, story)

	case story.Hours > 36 && story.Score < 50:
		r.vLowStories = append(r.vLowStories, story)

	case story.Hours > 24 && story.Score < 20:
		r.vLowStories = append(r.vLowStories, story)

	case story.Hours > 12 && story.Score < 10:
		r.vLowStories = append(r.vLowStories, story)

	case story.Score < 100 && story.ScoreAvg < 20:
		r.lowStories = append(r.lowStories, story)

	default:
		r.mainStories = append(r.mainStories, story)
	}
}

func logStories(r *results) {
	homeDir, err := os.UserHomeDir()
	errExit(err, "error: cannot get home dir")
	progDir := homeDir + "/.config/newsfilter/"
	_ = progDir

	storiesToFile(progDir, "hn_main.tsv", r.mainStories)
	storiesToFile(progDir, "hn_blocked.tsv", r.blockedStories)
	storiesToFile(progDir, "hn_vlow.tsv", r.vLowStories)
}

func storiesToFile(progDir, file string, stories []hnStory) {
	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY

	fd, err := os.OpenFile(progDir+file, fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fdIDs, err := os.OpenFile(progDir+"processed_ids", fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fdIDs.Close()

	for _, story := range stories {
		fmt.Fprintln(fd, logLine(story))
		fmt.Fprintln(fdIDs, story.ID)
	}
}

func logLine(story hnStory) string {
	return fmt.Sprintf("%s\t"+
		"%d\t"+
		"%s\t"+
		"%s\t"+
		"%s",
		story.Time.Format("2006-01-02"),
		story.ID,
		story.By,
		story.Title,
		story.Url,
	)
}

func prepareHtml(r *results) {
	homeDir, err := os.UserHomeDir()
	errExit(err, "error: cannot get home dir")
	progDir := homeDir + "/.config/newsfilter/"

	file := "news_xxx.html"
	fdOpts := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	fd, err := os.OpenFile(progDir+file, fdOpts, 0644)
	errExit(err, "error: cannot create file")
	defer fd.Close()

	fmt.Fprintln(fd, htmlHeader)
	for _, story := range r.mainStories {
		fmt.Fprintf(fd, "%s\n", story.Title)
		fmt.Fprintf(fd, "<a href='%s'>link</a>", story.Url)
		fmt.Fprintf(fd, " <a href='%s'>hn</a>",
			"https://news.ycombinator.com/item?id="+
				strconv.Itoa(story.ID))
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
        html, pre {     max-width: 750px;
                        margin: 0 auto;
                        line-height: 1.2;
                        color: #bbb;
                        background-color: #000;
                        -webkit-font-smoothing: none;
                        -webkit-text-stroke: 0.3px;
        }
        a {             color: #009900;
                        background-color: transparent;
                        text-decoration: underline;
        }
</style>

<body><pre>
`

var htmlFooter string = `</pre></body>
</html>`
