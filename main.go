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

	"net"
	"os/signal"
	"strings"
	"syscall"

	"sync"

	. "github.com/logrusorgru/aurora"
	"github.com/subosito/gotenv"
)

var pIndex = -1

type Program struct {
	name        string
	command     *exec.Cmd
	ignoreError bool
	Logs        bool
}

func NewProgram(script Script) Program {
	if !script.AbsPath {
		base, _ := os.Getwd()
		script.Path = base + script.Path
	}
	if _, err := os.Stat(script.Path); err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("File not exist:%s", script.Path))
		}
	}

	command := exec.Command(script.Path, script.Args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pIndex++
	return Program{script.Name, command, script.IgnoreError, script.Logs}
}

func isUrlAlive(url string) bool {
	_, err := resty.R().SetHeader("Accept", "application/file").
		Get(url)
	if err == nil {
		return true
	}
	return false
}

func isTcpAlive(tpcUrl string) bool {
	url := strings.Split(tpcUrl, "://")[1]
	conn, err := net.Dial("tcp", url)
	if err != nil {
		return false
	}
	conn.Close()
	return true

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

	p.command.Args = args

	// set the output to our variable
	p.command.Stdout = &out
	p.command.Stderr = &out

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
				if strings.Contains(health, "http") {
					if isUrlAlive(health) {
						break
					}
				}
				if strings.Contains(health, "tcp") {
					if isTcpAlive(health) {
						break
					}
				}
			} else {
				break
			}
		}
		time.Sleep(1 * time.Second)
		count++
	}

	if p.Logs {

		pIndex++
		go readLog(&out, p.name, pIndex)
	}
	if !bgMode {
		p.command.Wait()
	}

}

func (p *Program) kill() {
	pgid, err := syscall.Getpgid(p.command.Process.Pid)
	if err == nil {
		syscall.Kill(-pgid, 15) // note the minus sign
	}
}

type Task struct {
	Name            string
	EnvPath         string
	WaitFinish      bool
	CmdAfterExit    string
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
	Logs         bool
	Environments []map[string]string
	SleepAfter   time.Duration
}

func generateJson() {
	_, err := ioutil.ReadFile("go-task-runner.json")
	if err != nil {
		d1 := []byte("{\n\t\"task-hello\": {\n\t\t\"envPath\": \"\", \n\t\t\"environments\": [{\"DOMAIN\": \"WORLD\"}], \n\t\t\"scripts\": [\n\t\t\t{\n\t\t\t\t\"name\": \"Say Hello\",\n\t\t\t\t\"logs\": true,\n\t\t\t\t\"path\": \"/bin/sh\",\n\t\t\t\t\"absPath\": true,\n\t\t\t\t\"sleepAfter\": 1,\n\t\t\t\t\"timeout\": 10,\n\t\t\t\t\"args\": [\"\", \"-c\", \"echo Hello from $DOMAIN\"]\n\t\t\t}\n\t\t]\n\t}\n}")
		_ = ioutil.WriteFile("go-task-runner.json", d1, 0777)
	}
}

func main() {
	taskNames := os.Args

	if taskNames[1] == "init" {
		generateJson()
	}

	var tasks map[string]Task
	input, err := ioutil.ReadFile("go-task-runner.json")
	if err != nil {
		log.Fatal(err)
	}
	_ = json.Unmarshal(input, &tasks)

	programs := make(map[string]Program)
	var qtdTasks int
	for _, taskName := range taskNames {
		task := tasks[taskName]
		if task.CmdAfterExit != "" {
			go forceKill(task.CmdAfterExit)
		}
		var wg sync.WaitGroup
		programs, qtdTasks = runTask(task)
		wg.Add(qtdTasks)
		if task.WaitFinish {
			wg.Wait()
		}
	}
	exitPrograms(programs)
}

func runTask(task Task) (map[string]Program, int) {
	var qtdTasks int
	gotenv.Load(task.EnvPath)
	setEnvironment(task.Environments)
	programs := make(map[string]Program)

	for _, script := range task.Scripts {
		fmt.Println("Running:", script.Name)
		setEnvironment(script.Environments)
		program := NewProgram(script)
		if script.BgMode {
			qtdTasks++
		}
		program.Run(script.BgMode, script.Timeout, script.Args, script.HealthCheck)
		programs[script.Name] = program
		time.Sleep(script.SleepAfter * time.Second)
	}
	return programs, qtdTasks
}

func setEnvironment(envs []map[string]string) {
	for _, env := range envs {
		for key, value := range env {
			os.Setenv(key, value)
		}
	}
}

func exitPrograms(programs map[string]Program) {

	var appErrs []string
	for _, program := range programs {
		if program.command.ProcessState != nil {
			if !program.command.ProcessState.Success() && !program.ignoreError {
				appErrs = append(appErrs, program.name)
			}
		}
		program.kill()
	}

	if len(appErrs) != 0 {
		log.Fatal("Error in application found:", appErrs)
	}
}

func forceKill(cmd string) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		sig := <-sigchan
		// do anything you need to end program cleanly
		fmt.Println("Exited:", sig)
		command := exec.Command("/bin/sh", "-c", cmd)
		command.Run()
		os.Exit(0)
	}()
}
