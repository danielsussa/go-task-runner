package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/go-resty/resty"

	. "github.com/logrusorgru/aurora"
)

type Program struct {
	name    string
	command *exec.Cmd
	index   int
}

func NewProgram(name string, path string, args []string, index int) Program {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File not exist:%s", path))
		}
	}

	command := exec.Command(path, args...)
	return Program{name, command, index}
}

func isUrlAlive(url string) bool {
	_, err := resty.R().SetHeader("Accept", "application/file").
		Get(url)
	if err == nil {
		return true
	}
	return false
}

func getColors(colorNum int) Color {
	colorNum = colorNum % 7
	switch colorNum {
	case 0:
		return GreenFg
	case 1:
		return BlueFg
	case 2:
		return RedFg
	case 3:
		return BrownFg
	case 4:
		return MagentaFg
	case 5:
		return CyanFg
	case 6:
		return BlueFg
	default:
		return GrayFg
	}

}

func readLog(logB *bytes.Buffer, name string, index int) {
	color := getColors(index)
	for {
		logs, _ := logB.ReadString(1)
		if logs != "" {
			colorize := Colorize(name+": ", color)
			fmt.Print(colorize, logs)
		}
		time.Sleep(1 * time.Microsecond)
	}
}

func (p *Program) Run(bgMode bool, timeout int, args []string, health string) {
	var out bytes.Buffer

	// set the output to our variable
	p.command.Stdout = &out
	p.command.Stderr = &out
	go readLog(&out, p.name, p.index)

	p.command.Args = args

	if bgMode {
		go p.command.Run()
	} else {
		p.command.Run()
	}

	count := 0
	for {
		if count > timeout {
			log.Println("LOG:", p.command.Stdout)
			log.Println("ERR:", p.command.Stderr)
			log.Fatal("Not responding..")
			os.Exit(2)
		}
		if p.command.Process != nil {
			if health != "" {
				if isUrlAlive(health) {
					break
				}
			} else {
				break
			}
		}
		time.Sleep(1 * time.Second)
		count++
	}
}

func (p *Program) kill() {
	p.command.Process.Kill()
	for {
		if p.command.ProcessState != nil {
			if p.command.ProcessState.Exited() {
				break
			}
			p.command.Process.Kill()
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
}

type Scripts struct {
	Name        string
	Path        string
	HealthCheck string
	Args        []string
	BgMode      bool
	Timeout     int
	SleepAfter  time.Duration
}

type Environments struct {
	Key   string
	Value string
}

func main() {
	base, _ := os.Getwd()

	var scripts []Scripts
	input, err := ioutil.ReadFile("go-task-runner.json")
	if err != nil {
		log.Fatal(err)
	}
	_ = json.Unmarshal(input, &scripts)

	programs := make(map[string]Program)

	for i, script := range scripts {
		fmt.Println("Running:", script.Name)
		finalPath := base + script.Path
		program := NewProgram(script.Name, finalPath, script.Args, i)
		program.Run(script.BgMode, script.Timeout, script.Args, script.HealthCheck)
		programs[script.Name] = program
		time.Sleep(script.SleepAfter * time.Second)
	}

	var containErr bool

	for _, program := range programs {
		if program.command.ProcessState != nil {
			if program.command.ProcessState.Success() == false {
				containErr = true
			}
		}
		program.kill()
	}

	if containErr {
		log.Fatal("Error in application found!")
	}
}
