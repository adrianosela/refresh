# Refresh

Go library to keep short-lived credentials fresh with flexible strategies.

[![Go Report Card](https://goreportcard.com/badge/github.com/adrianosela/refresh)](https://goreportcard.com/report/github.com/adrianosela/refresh)
[![Documentation](https://godoc.org/github.com/adrianosela/refresh?status.svg)](https://godoc.org/github.com/adrianosela/refresh)
[![GitHub issues](https://img.shields.io/github/issues/adrianosela/refresh.svg)](https://github.com/adrianosela/refresh/issues)
[![license](https://img.shields.io/github/license/adrianosela/refresh.svg)](https://github.com/adrianosela/refresh/blob/master/LICENSE)

The idea is that you **only need to implement the logic for getting a new value**, and don't need to worry about the logic for keeping the value fresh (which typically involves reasoning about concurrency and strategizing on when to refresh the value).

### Usage:

```
// initialize a new Refresher with your own refresh function
refresher := refresh.NewRefresher(refreshStuffFunc)
defer refresher.Stop()

// block until there's a valid value available (or timeout)
err := refresher.WaitForInitialValue(timeout)
if err != nil {
	log.Fatalf("failed while waiting for initial value: %v", err)
}

// concurrency-safe getter
value := refresher.GetCurrent()
```