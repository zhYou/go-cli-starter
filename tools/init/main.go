// Command init renames a new copy of the template. It uses no external packages.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zhYou/go-cli-starter/internal/scaffold"
)

func main() {
	module := flag.String("module", "", "Go module path, for example github.com/acme/mycli")
	name := flag.String("name", "", "Executable name, for example mycli")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Error: unexpected positional arguments")
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	changed, err := scaffold.Initialize(root, *module, *name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized %s (%s); %d files updated.\n", *module, *name, len(changed))
}
