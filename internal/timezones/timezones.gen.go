//go:build generate

// Go does not provide a way to search through a list of timezones. This downloads the timezone database from IANA and
// Generates a source file that can be used as the timezone list.

package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/samber/lo"
)

func main() {
	fmt.Println("Generating timezone list.")
	f := lo.Must(os.Create(os.Getenv("GOFILE")))

	// Header
	fmt.Fprintln(f, `// Code generated by timezones.gen.go`)
	fmt.Fprintln(f, "//go:generate go run timezones.gen.go")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "package %s\n", os.Getenv("GOPACKAGE"))
	fmt.Fprintln(f)
	fmt.Fprintln(f, "var TimeZones = []string{")

	for _, url := range []string{
		"https://data.iana.org/time-zones/code/zone1970.tab",
		"https://data.iana.org/time-zones/tzdb/backward",
	} {
		data := lo.Must(http.Get(url))
		scanner := bufio.NewScanner(data.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			fmt.Fprintf(f, "  \"%s\",\n", strings.Fields(line)[2])
		}
	}

	fmt.Fprintln(f, "}")
	lo.Must0(f.Close())
}
