// dump-hn - dumps all Hacker News comments and stories to a text file
//
// info:
// output file is a tab delimited text file
// characters '\t', '\r' and '\n' are converted to '<_\t_>', '<_\r_>', '<_\n_>'
// out file: /tmp/hndump.tsv
// info on chunks not downloaded due to errors: /tmp/hndump_error_chunks.txt

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
	"sync"
	"time"
)

// LASTCHUNK must be divisable by CHUNKSIZE
const WORKERS = 128
const CHUNKSIZE = 100
const LASTCHUNK = 29750000 // 2021-12-31 18:00
var OUTFILE="/tmp/hndump.tsv"

var MU = &sync.Mutex{}

type hnItem struct {
	ID          int    `json:"id"`
	Deleted     bool   `json:"deleted"`
	Type        string `json:"type"`
	By          string `json:"by"`
	TimeI       int64  `json:"time"`
	Text        string `json:"text"`
	Dead        bool   `json:"dead"`
	Parent      int    `json:"parent"`
	Poll        int    `json:"poll"`
	Kids        []int  `json:"kids"`
	Url         string `json:"url"`
	Score       int    `json:"score"`
	Title       string `json:"title"`
	Parts       []int  `json:"parts"`
	Descendants int    `json:"descendants"`
}

type url struct {
	url string
	id  int
}

type result struct {
	items []hnItem
	err   error
}

func main() {
	var chunks []int
	fmt.Println("reading already processed chunks...")
	processedChunks := readChunks()
	for i := 0; i < LASTCHUNK/CHUNKSIZE; i++ {
		if ! intExists(processedChunks, i) {
			chunks = append(chunks, i)
		}
	}

	fmt.Println("getting the data...")
	fmt.Print("\033[s") // save the cursor position
	getItems(chunks)
	fmt.Println("\ndone.")
}

func readChunks() []int {
	var chunks = make(map[int64]bool)
	var res []int

	f, err := os.Open(OUTFILE)
	if err != nil {
		return res
	}
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		id, err := strconv.ParseInt(fields[0], 10, 64)
		errExit(err, "can't parse int: " + fields[0])
		chunk := id / CHUNKSIZE
		if id % CHUNKSIZE == 0 && id != 0 {
			chunk -= 1
		}
		chunks[chunk] = true
	}

	for chunk, _ := range chunks {
		res = append(res, int(chunk))
	}
	sort.Ints(res)

	return res
}

func intExists(s []int, el int) bool {
        i := sort.SearchInts(s, el)
        if i >= len(s) {
                return false
        }
        return s[i] == el
}

func queryItem(id int) (hnItem, error) {
	var item hnItem

	url := "https://hacker-news.firebaseio.com/v0/item/" +
		strconv.Itoa(id) + ".json"

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	ua := "Wget/1.20.3 (linux-gnu)"
	req.Header.Set("User-Agent", ua)
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return hnItem{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return hnItem{}, err
	}

	err = json.Unmarshal(body, &item)
	if err != nil {
		return hnItem{}, err
	}

	return item, nil
}

func queryItems(chunks <-chan int, resCh chan<- result, wg *sync.WaitGroup, w int) {
	defer wg.Done()
	for chunk := range chunks {
		var items []hnItem
		var chunkErr error

		chunkStart := chunk*CHUNKSIZE + 1
		chunkEnd := chunk*CHUNKSIZE + CHUNKSIZE

		for id := chunkStart; id <= chunkEnd; id++ {
			item, err := queryItem(id)
			if err != nil {
				chunkErr = err
				break
			}
			items = append(items, item)
		}

		if chunkErr != nil {
			MU.Lock()
			saveErrorChunk(chunk, chunkErr)
			MU.Unlock()
			continue
		}

		MU.Lock()
		fmt.Print("\033[u\033[K") // restore cursor pos and clear line
		fmt.Printf("worker %3d saving chunk %10d", w, chunk)
		MU.Unlock()

		resCh <- result{items, nil}
	}
}

func getItems(chunks []int) {
	var wg sync.WaitGroup

	inputCh := make(chan int)
	resCh := make(chan result)

	w := 0
	wg.Add(WORKERS)
	for i := 1; i <= WORKERS; i++ {
		w++
		go queryItems(inputCh, resCh, &wg, w)
	}

	go func() {
		for _, chunk := range chunks {
			inputCh <- chunk
		}
		close(inputCh)
	}()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	fd, err := os.OpenFile(OUTFILE, fdOpts, 0644)
	errExit(err, "error: cannot create a file")
	defer fd.Close()

	for r := range resCh {
		MU.Lock()
		for _, item := range r.items {
			fmt.Fprintln(fd, logHnLine(item))
		}
		MU.Unlock()
	}
}

func saveErrorChunk(chunk int, chunkErr error) {
	fdOpts := os.O_CREATE | os.O_APPEND | os.O_WRONLY
	fd, err := os.OpenFile("/tmp/hndump_error_chunks.txt", fdOpts, 0644)
	errExit(err, "error: cannot create a file")
	defer fd.Close()

	fmt.Fprintln(fd, strconv.Itoa(chunk), "\t", chunkErr)
}


func sepReplace(s string) string {
	res := strings.Replace(s, "\t", "<_\\t_>", -1)
	res = strings.Replace(res, "\n", "<_\\n_>", -1)
	res = strings.Replace(res, "\r", "<_\\r_>", -1)

	return res
}

func logHnLine(item hnItem) string {
	t := time.Unix(item.TimeI, 0)
	var kids, sep string
	for _, k := range item.Kids {
		kids += sep + strconv.Itoa(k)
		sep = ","
	}
	var parts string
	sep = ""
	for _, p := range item.Parts {
		parts += sep + strconv.Itoa(p)
		sep = ","
	}

	return fmt.Sprintf(
		"%d\t"+
			"%s\t"+
			"%2.2d:%2.2d\t"+
			"%d\t"+
			"%s\t"+
			"%s\t"+
			"%s\t"+

			"%s\t"+
			"%d\t"+
			"%d\t"+
			"%d\t"+
			"%d\t"+
			"%s\t"+
			"%s\t"+
			"%s\t"+
			"%s\t"+
			"%s",
		item.ID,
		t.Format("2006-01-02"),
		t.Hour(), t.Minute(),
		item.TimeI,
		item.Type,
		strconv.FormatBool(item.Deleted),
		strconv.FormatBool(item.Dead),

		sepReplace(item.By),
		item.Score,
		item.Descendants,
		item.Parent,
		item.Poll,
		kids,
		parts,
		sepReplace(item.Title),
		sepReplace(item.Url),
		sepReplace(item.Text),
	)
}

func errExit(err error, msg string) {
	if err != nil {
		log.Println("\n * " + msg)
		log.Fatal(err)
	}
}
