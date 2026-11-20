package main

import (
	"bufio"
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

type Result struct {
	url        string
	statusCode int
	err        error
	duration   time.Duration
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	workerCount := flag.Int("workers", 5, "number of workers")
	timeout := flag.Duration("timeout", 5*time.Second, "request timeout")
	filePath := flag.String("file", "", "file containing URLs (one per line)")
	flag.Parse()

	var targets []string
	if *filePath != "" {
		loaded, err := loadURLs(*filePath)
		if err != nil {
			logger.Error("failed to read URL file", slog.String("error", err.Error()))
			os.Exit(1)
		}
		targets = loaded
	} else {
		targets = flag.Args()
	}

	if len(targets) == 0 {
		logger.Error("provide at least one URL via arguments or -file")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &http.Client{Timeout: *timeout}
	workersLimit := *workerCount
	if workersLimit > len(targets) {
		workersLimit = len(targets)
	}
	if workersLimit < 1 {
		workersLimit = 1
	}

	urls := make(chan string)
	results := make(chan Result)
	var workers sync.WaitGroup

	for i := 0; i < workersLimit; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case url, ok := <-urls:
					if !ok {
						return
					}
					res := checkURL(ctx, client, url)
					select {
					case <-ctx.Done():
						return
					case results <- res:
					}
				}
			}
		}()
	}

	go func() {
		defer close(urls)
		for _, url := range targets {
			select {
			case <-ctx.Done():
				return
			case urls <- url:
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	total := 0
	success := 0
	failed := 0

	for res := range results {
		total++
		if res.err != nil {
			failed++
			logger.Error("URL check failed",
				slog.String("url", res.url),
				slog.String("error", res.err.Error()),
				slog.Duration("duration", res.duration),
			)
		} else {
			success++
			logger.Info("URL check completed",
				slog.String("url", res.url),
				slog.Int("status", res.statusCode),
				slog.Duration("duration", res.duration),
			)
		}
	}

	logger.Info("summary",
		slog.Int("total", total),
		slog.Int("success", success),
		slog.Int("failed", failed),
	)
}

func loadURLs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var list []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		list = append(list, line)
	}
	return list, scanner.Err()
}

func checkURL(ctx context.Context, client *http.Client, url string) Result {
	start := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{
			url:      url,
			err:      err,
			duration: time.Since(start),
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return Result{
			url:      url,
			err:      err,
			duration: time.Since(start),
		}
	}
	defer response.Body.Close()

	return Result{
		url:        url,
		statusCode: response.StatusCode,
		duration:   time.Since(start),
	}
}
