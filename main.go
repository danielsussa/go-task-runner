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
	"github.com/subosito/gotenv"
	"os/signal"
	"syscall"
)

type Program struct {
	name        string
	command     *exec.Cmd
	index       int
	ignoreError bool
}

func NewProgram(name string, path string, args []string, index int, ignoreError bool) Program {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File not exist:%s", path))
		}
	}

	command := exec.Command(path, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return Program{name, command, index, ignoreError}
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

	p.command.Args = args

	p.command.Start()

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
	go readLog(&out, p.name, p.index)
	if !bgMode {
		p.command.Wait()
	}
}

func (p *Program) kill() {
	syscall.Kill(-p.command.Process.Pid, 15)
}

type Task struct {
	Name            string
	EnvPath         string
	TaskIfInterrupt []string
	Environments    []map[string]string
	Scripts         []Script
}

type Script struct {
	Name         string
	Path         string
	AbsPath      bool
	HealthCheck  string
	Args         []string
	BgMode       bool
	Timeout      int
	IgnoreError  bool
	Environments []map[string]string
	SleepAfter   time.Duration
}

func main() {
	taskNames := os.Args

	base, _ := os.Getwd()

	var tasks []Task
	input, err := ioutil.ReadFile("go-task-runner.json")
	if err != nil {
		log.Fatal(err)
	}
	_ = json.Unmarshal(input, &tasks)

	programs := make(map[string]Program)
	forcedExit(&programs)

	for _, taskName := range taskNames {
		for _, task := range tasks {
			gotenv.Load(task.EnvPath)
			setEnvironment(task.Environments)
			if taskName == task.Name {
				for i, script := range task.Scripts {
					fmt.Println("Running:", script.Name)
					finalPath := base + script.Path
					if script.AbsPath {
						finalPath = script.Path
					}
					setEnvironment(script.Environments)
					program := NewProgram(script.Name, finalPath, script.Args, i, script.IgnoreError)
					program.Run(script.BgMode, script.Timeout, script.Args, script.HealthCheck)
					programs[script.Name] = program
					time.Sleep(script.SleepAfter * time.Second)
				}
			}
		}
	}

	exitPrograms(programs)
}

func runTask() {

}

func setEnvironment(envs []map[string]string) {
	for _, env := range envs {
		for key, value := range env {
			os.Setenv(key, value)
		}
	}
}

func exitPrograms(programs map[string]Program) {
	var containErr bool

	for _, program := range programs {
		if program.command.ProcessState != nil {
			if !program.command.ProcessState.Success() && !program.ignoreError {
				containErr = true
			}
		}
		program.kill()
	}

	if containErr {
		log.Fatal("Error in application found!")
	}
}

func forcedExit(programs *map[string]Program) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigchan
		// do anything you need to end program cleanly
		log.Println("FORCE EXIT!", s)
		exitPrograms(*programs)
		os.Exit(0)
	}()
}
