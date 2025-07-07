package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type result struct {
	code int
	url  string
}

func push(url string, websiteURL string, client *http.Client) (int, string) {
	urlNew := strings.Replace(url, "{url}", websiteURL, -1)
	req, err := http.NewRequest("GET", urlNew, nil)
	if err != nil {
		return 500, urlNew
	}
	//
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 500, urlNew
	}
	defer resp.Body.Close()
	return resp.StatusCode, urlNew
}

func worker(id int, websiteURL string, client *http.Client, jobs <-chan string, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()
	for url := range jobs {
		code, urlNew := push(url, websiteURL, client)
		results <- result{code: code, url: urlNew}
	}
}

func code(codeDict map[string]int, code int) {
	switch {
	case code < 300:
		codeDict["2xx"]++
	case code < 400:
		codeDict["3xx"]++
	case code < 500:
		codeDict["4xx"]++
	default:
		codeDict["5xx"]++
	}
}

func main() {
	fmt.Println("外链分发工具 v1.1 - Go Version")
	fmt.Println(strings.Repeat("-", 16))

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("输入你的网站（不要带http）：")
	websiteURL, _ := reader.ReadString('\n')
	websiteURL = strings.TrimSpace(websiteURL)

	fmt.Print("请输入工作线程（不写则默认为16，建议不超过CPU核心数的10倍）:")
	workersStr, _ := reader.ReadString('\n')
	workersStr = strings.TrimSpace(workersStr)

	workers := 16
	maxWorkers := runtime.NumCPU() * 10
	if workersStr != "" {
		if val, err := strconv.Atoi(workersStr); err == nil {
			if val > 0 && val <= maxWorkers {
				workers = val
			} else if val > maxWorkers {
				fmt.Printf("线程数过高，已限制为%d\n", maxWorkers)
				workers = maxWorkers
			}
		}
	}
	fmt.Printf("实际工作线程数：%d\n", workers)
	fmt.Println(strings.Repeat("-", 16))

	startTime := time.Now()

	file, err := os.Open("links.txt")
	if err != nil {
		fmt.Printf("打开 links.txt 失败: %v\n", err)
		fmt.Println("\n\n按任意键退出……")
		reader.ReadString('\n')
		return
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("读取 links.txt 失败: %v\n", err)
		fmt.Println("\n\n按任意键退出……")
		reader.ReadString('\n')
		return
	}

	allCount := len(urls)
	count := 0
	codeReport := map[string]int{
		"2xx": 0,
		"3xx": 0,
		"4xx": 0,
		"5xx": 0,
	}

	jobs := make(chan string, allCount)
	results := make(chan result, allCount)

	//
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: tr}

	var wg sync.WaitGroup

	for w := 1; w <= workers; w++ {
		wg.Add(1)
		go worker(w, websiteURL, client, jobs, results, &wg)
	}

	for _, url := range urls {
		jobs <- url
	}
	close(jobs)

	//
	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		count++
		code(codeReport, res.code)
		if count%100 == 0 || count == allCount {
			fmt.Printf("%.2f%% >>> %d / %d >>> %d %s\n", float64(count)/float64(allCount)*100, count, allCount, res.code, res.url)
		}
	}

	fmt.Println("\n>>> 已全部分发完毕~")
	fmt.Println(">>> 各个状态码的情况分布：")
	for k, v := range codeReport {
		fmt.Printf("%s：%d\n", k, v)
	}

	fmt.Printf(">>> 运行时间：%.2f秒\n", time.Since(startTime).Seconds())

	fmt.Println("\n\n按任意键退出……")
	reader.ReadString('\n')
}
