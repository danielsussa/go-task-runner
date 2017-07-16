# Go-Task-Runner

A simple task manager to execute applications and manage proccess.

### Installation

Requiered [Go](https://nodejs.org/) v1.8+ to run.


```json
$ go get github.com/danielsussa/go-task-runner
$ go-task-runner init
```
This will generate a new go-task-runner.json file:

```json
[
	{
		"name":"Task Hello World",
		"envPath":"",
		"environments":[{"DOMAIN":"WORLD"}],
		"scripts":[
			{
				"name":"Say Hello",
				"path":"/bin/sh", 
				"absPath":true, 
				"timeout":10,
				"args":["","-c","echo Hello $DOMAIN"]
			}
		]
	}
]
```
- Tasks properties:
	- name: task name
	- envPath: You can use a env file containing:
		-  export FOO=BAR
		- FOO=BAR (*.env)
	- environments: List of key value environment variable
	- script: List of scripts...
- Scripts
	- name: script name
	- path: The path of bin, exec...
	- absPath: If true the path is pointing to the root folder, else to the go-task-runner folder
	- healthCheck: You can pass a http or tcp url to do a helthcheck. The step only start when url respond. *(ex: "healthCheck":"http://localhost:9200", "healthCheck":"tcp://localhost:2181")*
	- bgMode: if true this script will run in background mode
	- timeout: time to wait during healthcheck
	- ignoreError: If false, when error occurs the exit of go-task-runner will be 1
	- sleepAfter: sleep time in seconds to sleep after script has executed
	- args:  list of args to pass
	- logs: if true show logs in terminal


## New Features!

- Multi tasks, select using **go-task-runner** {task-name}
- Multi scripts per task, you can run shell commands or exec, bin files and pass arguments
- Multi-color logs for multi tasks