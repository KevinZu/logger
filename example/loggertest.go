package main

import (
	"errors"
	"flag"
	"github.com/KevinZu/logger"
	"time"
)

func doSomething() error {
	return errors.New("hahahahahahah")
}

const logPath = "E:\\log\\example.log"

var verbose = flag.Bool("verbose", false, "print info level logs to stdout")

func main() {

	logger.LoggerInit("", "test_", *verbose, true)
	for {

		logger.Info("I'm about to do something!")
		if err := doSomething(); err != nil {
			logger.Errorf("Error running doSomething: %v", err)
		}
		time.Sleep(time.Duration(3) * time.Second)
	}

}
