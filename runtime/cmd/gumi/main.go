// Command gumi is the Gumi Runtime entrypoint. It delegates to the CLI package,
// which parses arguments and dispatches to the appropriate command.
package main

import "github.com/EffNine/gumi/runtime/internal/cli"

func main() {
	cli.Execute()
}
