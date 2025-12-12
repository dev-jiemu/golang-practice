//go:build darwin

package main

/*
#cgo CFLAGS: -I/usr/local/onnxruntime-osx-arm64-1.18.1/include
#cgo LDFLAGS: -L/usr/local/onnxruntime-osx-arm64-1.18.1/lib -lonnxruntime -Wl,-rpath,/usr/local/onnxruntime-osx-arm64-1.18.1/lib
*/
import "C"
