// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"flag"
	"github.com/gin-gonic/gin"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner struct {
	cmd       string
	stdout    string
	stderr    string
	completed bool
	exitCode  int
}

func newRuner(cmd string) *Runner {
	return &Runner{cmd: cmd}
}

func (r *Runner) Start() {
	go func() {
		c := exec.Command("bash", "-c", r.cmd)
		outBuff := bytes.NewBuffer([]byte{})
		errBuff := bytes.NewBuffer([]byte{})
		stdout := io.MultiWriter(outBuff, os.Stdout)
		stderr := io.MultiWriter(errBuff, os.Stderr)
		c.Stdout = stdout
		c.Stderr = stderr
		err := c.Run()
		if err != nil {
			code := 255
			errMessage := "unknown error"
			var ee *exec.ExitError
			ok := errors.As(err, &ee)
			if ok {
				code = ee.ExitCode()
				errMessage = errBuff.String()
			}
			r.exitCode = code
			r.stderr = errMessage
			r.stdout = outBuff.String()
		} else {
			r.exitCode = 0
			r.stdout = outBuff.String()
		}
		r.completed = true
	}()
}

func (r *Runner) GetStatus() *Status {
	return &Status{
		Command:   r.cmd,
		Completed: r.completed,
		Stdout:    r.stdout,
		Stderr:    r.stderr,
		ExitCode:  r.exitCode,
	}
}

func main() {
	var listenAddr string

	flag.StringVar(&listenAddr, "listen-address", ":8080", "The address the rest server binds to.")
	flag.Parse()
	cmd := strings.Join(flag.Args(), " ")
	r := newRuner(cmd)
	r.Start()
	router := gin.Default()
	router.GET("/status", func(context *gin.Context) {
		s := r.GetStatus()
		context.JSON(200, s)
		return
	})
	router.POST("/shutdown", func(context *gin.Context) {
		os.Exit(0)
	})

	router.Run(listenAddr)
}

type Status struct {
	Command   string `json:"command"`
	Completed bool   `json:"completed"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exitCode"`
}
