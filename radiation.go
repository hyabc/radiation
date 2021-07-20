package main

import (
	"fmt"
	"os"
	"io"
	"path"
	"os/exec"
	"strconv"
	"strings"
	"log"
	"sync"
	"encoding/json"
	"net/http"
	"golang.org/x/term"
)

const (
	entrylist_url = "/v1/entries?status=unread&direction=desc"
	entry_url = "/v1/entries"
	config_filename = ".radiation"
	max_retries = 5
)

var (
	config Config
	entry_list *EntryList
	article *Article
	wg sync.WaitGroup
)

type Entry struct {
	Id int
	Title string
	Url string
	Content string
}

type EntryUpdate struct {
	Entry_ids []int
	Status string
}

type EntryList struct {
	Total int
	Position int
	Entries []Entry
}

type Config struct {
	Token string
	Server_url string
	Page_entries int
	Lines int
}

type Article struct {
	Lines []string
	Title string
	Position int
}

func GetRequestOnce(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", config.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d\n", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func PutRequestOnce(url, content, content_type string) ([]byte, error) {
	req, err := http.NewRequest("PUT", url, strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", config.Token)
	req.Header.Set("Content-Type", content_type)
	req.Header.Set("Content-Length", strconv.Itoa(len(content)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("HTTP error %d\n", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func GetRequest(url string) (data []byte, err error) {
	for count := 0; count < max_retries; count++ {
		data, err = GetRequestOnce(url)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("retrying failed %s", err)
}

func PutRequest(url, content, content_type string) (data []byte, err error) {
	for count := 0; count < max_retries; count++ {
		data, err = PutRequestOnce(url, content, content_type)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("retrying failed %s", err)
}

func ReadConfig() (error) {
	loc := path.Join(os.Getenv("HOME"), config_filename)
	file, err := os.Open(loc)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return err
	}
	return nil
}

func GetEntryList() (*EntryList, error) {
	data, err := GetRequest(config.Server_url + entrylist_url)
	if err != nil {
		return nil, err
	}
	var list EntryList
	err = json.Unmarshal(data, &list)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func GetEntry(id int) (*Entry, error) {
	data, err := GetRequest(config.Server_url + entry_url + "/" + strconv.Itoa(id))
	if err != nil {
		return nil, err
	}
	var ent Entry
	err = json.Unmarshal(data, &ent)
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

func HtmlConvert(html string) (string, error) {
	cmd := exec.Command("lynx", "-dump", "-nolist", "-stdin")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	_, err = stdin.Write([]byte(html))
	stdin.Close()
	if err != nil {
		return "", err
	}
	text, err := io.ReadAll(stdout)
	stdout.Close()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}
	return string(text), nil
}

func PrintEntry(num int) (string, string, int) {
	if !(0 <= num && num < len(entry_list.Entries)) {
		return "", "Requested article out of bound\n", 0
	}
	ent := &entry_list.Entries[num]
	title := ent.Title
	id := ent.Id

	text, err := HtmlConvert(ent.Content)
	if err != nil {
		text = fmt.Sprintf("html conversion error: %s", err)
	}

	entry_list.Entries = append(entry_list.Entries[:num], entry_list.Entries[num + 1:]...)
	if len(entry_list.Entries) == 0 {
		entry_list.Position = 0;
	} else if entry_list.Position * config.Page_entries >= len(entry_list.Entries) {
		entry_list.Position--;
	}

	return title, text, id
}

func PrintEntryList() string {
	if len(entry_list.Entries) == 0 {
		return "Empty entry list\n"
	}
	var str string
	for index := 0; index < config.Page_entries; index++ {
		pos := index + entry_list.Position * config.Page_entries;
		if pos >= len(entry_list.Entries) {
			break
		}
		line := fmt.Sprintf("%d. %s\n", pos, entry_list.Entries[pos].Title)
		str += line
	}
	return str
}

func PrintHelpMsg() string {
	var str string
	str += "#:\tPrint article #\n"
	str += "list:\tList unread articles\n"
	str += "prev:\tSwitch to previous page\n"
	str += "next:\tSwitch to next page\n"
	str += "help:\tShow help message\n"
	str += "quit:\tQuit Project Radiation\n"
	return str
}

func PrintEntryHelpMsg() string {
	var str string
	str += "continue:\tContinue article\n"
	str += "help:\tShow help message\n"
	str += "quit:\tQuit this article and return\n"
	return str
}

func RefreshEntryList() string {
	var err error
	entry_list, err = GetEntryList()
	if err != nil {
		return fmt.Sprintf("Refresh entrylist failed: %s\n", err)
	}
	return PrintEntryList()
}

func SwitchEntryListNext() string {
	if (entry_list.Position + 1) * config.Page_entries < len(entry_list.Entries) {
		entry_list.Position++
		return PrintEntryList()
	} else {
		return PrintEntryList() + "Already at last page\n"
	}
}

func SwitchEntryListPrev() string {
	if entry_list.Position - 1 >= 0 {
		entry_list.Position--
		return PrintEntryList()
	} else {
		return PrintEntryList() + "Already at first page\n"
	}
}

func SwitchArticleNext() string {
	if (article.Position + 1) * config.Lines < len(article.Lines) {
		article.Position++
		return PrintArticleSection()
	} else {
		return PrintArticleSection() + "Already at last page\n"
	}
}

func SwitchArticlePrev() string {
	if article.Position - 1 >= 0 {
		article.Position--;
		return PrintArticleSection()
	} else {
		return PrintArticleSection() + "Already at first page\n"
	}
}

func PrintArticleSection() string {
	begin := article.Position * config.Lines
	end := (article.Position + 1) * config.Lines
	if len(article.Lines) < end {
		end = len(article.Lines)
	}
	return article.Title + "\n\n" + strings.Join(article.Lines[begin:end], "\n") + "\n"
}

func ProcessInput(t *term.Terminal, req string) bool {
	//Test numerical input
	num, conv_err := strconv.Atoi(req)
	if req == "" {
		conv_err = nil
		num = entry_list.Position * config.Page_entries
	}
	if conv_err == nil {
		title, text, id := PrintEntry(num)
		article = &Article{Title: title, Lines: strings.Split(text, "\n"), Position: 0}

		//Print content
		t.Write([]byte(PrintArticleSection()))
		if article != nil {
			prompt := fmt.Sprintf("(%d) > ", id)
			t.SetPrompt(prompt)
		}

		//Mark read
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := MarkEntryRead(id)
			if err != nil {
				fmt.Printf("Error marking %d read: %s\n", id, err)
			}
		}(id)

		return true
	}

	//Test command
	switch req {
	case "r", "refresh":
		t.Write([]byte(RefreshEntryList()))
	case "l", "list":
		t.Write([]byte(PrintEntryList()))
	case "n", "next":
		t.Write([]byte(SwitchEntryListNext()))
	case "p", "prev", "previous":
		t.Write([]byte(SwitchEntryListPrev()))
	case "h", "help", "?":
		t.Write([]byte(PrintHelpMsg()))
	case "q", "quit":
		return false
	default:
		t.Write([]byte("Unknown command\n"))
	}
	return true
}

func ProcessInputEntry(t *term.Terminal, req string) {
	switch req {
	case "", "c", "continue", "n", "next":
		t.Write([]byte(SwitchArticleNext()))
		if (article.Position + 1) * config.Lines >= len(article.Lines) {
			article = nil
			t.SetPrompt("> ")
		}
	case "p", "prev", "previous":
		t.Write([]byte(SwitchArticlePrev()))
	case "q", "quit":
		article = nil
		t.SetPrompt("> ")
	case "h", "help", "?":
		t.Write([]byte(PrintEntryHelpMsg()))
	default:
		t.Write([]byte("Unknown command\n"))
	}
}

func MarkEntryRead(id int) error {
	url := config.Server_url + entry_url
	t := &EntryUpdate{Entry_ids: []int{id}, Status: "read"}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = PutRequest(url, string(data), "application/json")
	if err != nil {
		return err
	}
	return nil
}

func main() {
	//Read config
	err := ReadConfig()
	if err != nil {
		log.Fatalf("configuration file error: %s", err)
	}

	//Set terminal to raw mode
	old_term, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("terminal config error: %s", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), old_term)
	t := term.NewTerminal(os.Stdin, "> ")

	defer wg.Wait()

	//Generate entrylist
	t.Write([]byte(RefreshEntryList()))

	for {
		//Readline and clear screen
		var req string
		req, err = t.ReadLine()
		t.Write([]byte("\033\143"))
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Fatalf("read command error: %s", err)
		}

		if article == nil {
			cont := ProcessInput(t, req)
			if !cont {
				return
			}
		} else {
			ProcessInputEntry(t, req)
		}
	}
}
