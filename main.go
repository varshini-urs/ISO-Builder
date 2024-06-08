package main

import (
	"fmt"
	"syscall/js"
)

func main() {
	fmt.Println("WASM Go Initialized")

	js.Global().Set("downloadFiles", js.FuncOf(func(this js.Value, p []js.Value) interface{} {
		go downloadFiles()
		return nil
	}))

	select {} // This keeps the main function running
}

func downloadFiles() {
	fmt.Println("Download initiated")

	promise := js.Global().Get("fetch").Invoke("http://localhost:8081/download")
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) == 0 {
			js.Global().Get("console").Call("log", "Empty response")
			return nil
		}
		resp := args[0]
		if !resp.Get("ok").Bool() {
			js.Global().Get("console").Call("log", "Request failed with status:", resp.Get("status").Int())
			return nil
		}
		return resp.Get("arrayBuffer").Invoke()
	})).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("log", "Files downloaded successfully")
		fmt.Println("Download completed")
		// Further processing of data if needed
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("log", "Error making request:", args[0])
		return nil
	}))
}
