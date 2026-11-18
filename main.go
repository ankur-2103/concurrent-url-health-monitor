package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) < 2 {
		logger.Error("provide at least one URL")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, url := range os.Args[1:] {
		checkURL(client, logger, url)
	}
}

func checkURL(client *http.Client, logger *slog.Logger, url string) {
	start := time.Now()
	response, err := client.Get(url)
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
