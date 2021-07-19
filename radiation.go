package main

import (
	"fmt"
	"os"
	"io"
	"path"
	"os/exec"
	"strconv"
	"log"
	"encoding/json"
	"net/http"
)

const (
	entrylist_url = "/v1/entries?status=unread&direction=desc"
	entry_url = "/v1/entries"
	config_filename = ".radiation"
	max_retries = 5
)

var (
	config Config
)

type Entry struct {
	Id int
	Title string
	Url string
	Content string
}

type EntryList struct {
	Total int
	Entries []Entry
}

type Config struct {
	Token string
	Server_url string
}

func RequestOnce(url string) ([]byte, error) {
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

func Request(url string) (data []byte, err error) {
	for count := 0; count < max_retries; count++ {
		data, err = RequestOnce(url)
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
	data, err := Request(config.Server_url + entrylist_url)
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
	data, err := Request(config.Server_url + entry_url + "/" + strconv.Itoa(id))
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

func main() {
	err := ReadConfig()
	if err != nil {
		log.Fatalf("configuration file error: %s", err)
	}
	entry_list, err := GetEntryList()
	if err != nil {
		log.Fatalf("error getting entrylist: %s", err)
	}
	for i := 0;i < 10;i++ {
		fmt.Println(entry_list.Entries[i].Id, entry_list.Entries[i].Title)
	}
	ent, err := GetEntry(entry_list.Entries[0].Id);
	if err != nil {
		log.Fatalf("error getting entry %d: %s", 0, err)
	}

	fmt.Println(ent.Title)
	text, err := HtmlConvert(ent.Content)
	if err != nil {
		log.Fatalf("html conversion error: %s", err)
	}
	fmt.Println(text)
}

