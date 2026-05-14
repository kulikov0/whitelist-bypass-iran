//go:build !linux

package main

import "os"

func watchStdinQuit(_ chan<- os.Signal) {}
