package builder

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func CheckForNewChange(filename string, fileChange chan bool) error {
	initialStat, err := os.Stat(filename)
	if err != nil {
		return err
	}

	for {
		newStat, err := os.Stat(filename)
		if err != nil {
			return err
		}
		if newStat.ModTime() != initialStat.ModTime() {
			initialStat = newStat
			fileChange <- true
		}
		time.Sleep(1 * time.Second)
	}
}

func BuildOnChange(filepath string) {
	splitFilePath := strings.Split(filepath, "/")
	n := len(splitFilePath)
	filename := splitFilePath[n-1]
	dir := strings.Join(splitFilePath[:n-1], "/")

	os.Chdir(dir)

	fileChange := make(chan bool)
	log.Printf("[INFO] : Listening for changes on file: %s\n", filepath)

	go func(filename string, fileChange chan bool) {
		err := CheckForNewChange(filename, fileChange)
		if err != nil {
			log.Printf("[ERROR] while checking for new change: %s\n", err)
		}
	}(filename, fileChange)

	for {
		log.Printf("[INFO]: Building file %s\n", filepath)
		sigChange := make(chan bool, 1)
		go runCommand(filename, sigChange)
		<-fileChange
		sigChange <- true
		<-sigChange
	}
}

func runCommand(filename string, change chan bool) {

	cmd := exec.Command("go", "build", filename)
	err := cmd.Run()
	if err != nil {
		log.Println("[ERROR] Building the file :", err)
		return
	}

	filenameWithoutExtension := strings.Split(filename, ".")[0]

	cmd = exec.Command(fmt.Sprintf("./%s", filenameWithoutExtension))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("[ERROR] creating pipe:", err)
		return
	}
	defer stdout.Close()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println("[ERROR] creating pipe:", err)
		return
	}
	defer stdin.Close()

	reader := io.Reader(stdout)
	go func() {
		io.Copy(os.Stdout, reader)
	}()

	err = cmd.Start()
	if err != nil {
		log.Println("[ERROR] Starting command:", err)
		return
	}

	<-change

	cmd = exec.Command("kill", "-USR1", fmt.Sprint(cmd.Process.Pid))
	err = cmd.Run()
	if err != nil {
		log.Println("[ERROR] Killing process:", err)
		return
	}

	change <- true
}
