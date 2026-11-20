package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) < 2 {
		logger.Error("provide at least one URL")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &http.Client{Timeout: 5 * time.Second}
	workerCount := 5
	if workerCount > len(os.Args)-1 {
		workerCount = len(os.Args) - 1
	}

	urls := make(chan string)
	var workers sync.WaitGroup

	for i := 0; i < workerCount; i++ {
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
					checkURL(ctx, client, logger, url)
				}
			}
		}()
	}

	for _, url := range os.Args[1:] {
		select {
		case <-ctx.Done():
			close(urls)
			workers.Wait()
			return
		case urls <- url:
		}
	}
	close(urls)
	workers.Wait()
}

func checkURL(ctx context.Context, client *http.Client, logger *slog.Logger, url string) {
	start := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Error("URL check failed",
			slog.String("url", url),
			slog.String("error", err.Error()),
			slog.Duration("duration", time.Since(start)),
		)
		return
	}

	response, err := client.Do(request)
	if err != nil {
		logger.Error("URL check failed",
			slog.String("url", url),
			slog.String("error", err.Error()),
			slog.Duration("duration", time.Since(start)),
		)
		return
	}
	defer response.Body.Close()

	logger.Info("URL check completed",
		slog.String("url", url),
		slog.Int("status", response.StatusCode),
		slog.Duration("duration", time.Since(start)),
	)
}
