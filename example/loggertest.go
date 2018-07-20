package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/logger"
	"io"
	"os"
)

var iLogs []io.Writer

func intWriter(w io.Writer) {
	iLogs = []io.Writer{w}
}

func insterWriter(w io.Writer) {
	iLogs = append(iLogs, w)
}

func checkWriter(logs []io.Writer, writer io.Writer) int {
	for i, w := range logs {
		fmt.Println("iLogs[", i, "] =", w)
		if w == writer {
			return i
		}
	}

	return -1
}

func doSomething() error {
	return errors.New("hahahahahahah")
}

const logPath = "E:\\log\\example.log"
const logPath1 = "E:\\log\\example_1.log"

var verbose = flag.Bool("verbose", false, "print info level logs to stdout")

func main() {
	flag.Parse()

	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}

	//intWriter(lf)

	//if checkWriter(iLogs, lf) >= 0 {
	//	fmt.Println("===================")
	//} else {
	//	fmt.Println("--------------------")
	//}

	defer lf.Close()

	defer logger.Init("LoggerExample", *verbose, true, lf).Close()

	logger.Info("I'm about to do something!")
	if err := doSomething(); err != nil {
		logger.Errorf("Error running doSomething: %v", err)
	}

	zf, e := os.OpenFile(logPath1, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if e != nil {
		logger.Fatalf("===Failed to open log file: %v", err)
	}

	if logger.ReplaceWriter(lf, zf) {
		fmt.Println("okokok")
	} else {
		fmt.Println("ererer")
	}

	//lf.Close()

	defer zf.Close()

	logger.Info("I'm about to do something!")
	if err := doSomething(); err != nil {
		logger.Errorf("Error running doSomething: %v", err)
	}

}
