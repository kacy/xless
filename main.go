package main

import "github.com/kacyfortner/ios-build-cli/cmd"

var version = "dev"

func main() {
	cmd.Execute(version)
}
