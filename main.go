package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

type Program struct {
	name    string
	command *exec.Cmd
}

func NewProgram(name string, path string, args []string) Program {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File not exist:%s", path))
		}
	}

	command := exec.Command(path, args...)
	return Program{name, command}
}

func isUrlAlive(url string) bool {
	_, err := resty.R().SetHeader("Accept", "application/file").
		Get(url)
	if err == nil {
		return true
	}
	return false
}

func (p *Program) Run(bgMode bool, timeout int, args []string, health string) {
	var out bytes.Buffer

	// set the output to our variable
	p.command.Stdout = &out

	p.command.Args = args

	if bgMode {
		go p.command.Run()
	} else {
		p.command.Run()
	}

	count := 0
	for {
		if count > timeout {
			log.Println("Not responding..")
			log.Println("LOG:", p.command.Stdout)
			log.Println("ERR:", p.command.Stderr)
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

	for _, script := range scripts {
		fmt.Println("Running:", script.Name)
		finalPath := base + script.Path
		program := NewProgram(script.Name, finalPath, script.Args)
		program.Run(script.BgMode, script.Timeout, script.Args, script.HealthCheck)
		programs[script.Name] = program
	}

	var containErr bool

	for _, program := range programs {

		log.Println("LOG", program.name, ":", program.command.Stdout)
		log.Println("Status exit", program.name, ":", program.command.Stderr)

		if program.command.ProcessState != nil {
			if program.command.ProcessState.Success() == false {
				log.Println("Error in: ", program.name)
				containErr = true
			}
		}
		program.kill()
	}

	if containErr {
		log.Fatal("Error in application found!")
	}
}