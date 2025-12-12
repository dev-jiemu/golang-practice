//go:build linux

package main

/*
#cgo linux CFLAGS: -I/usr/local/onnxruntime/include
#cgo linux LDFLAGS: -L/usr/local/onnxruntime/lib -lonnxruntime -Wl,-rpath,/usr/local/onnxruntime/lib
*/
import "C"
