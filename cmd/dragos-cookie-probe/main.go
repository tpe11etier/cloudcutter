// Diagnostic for the browser-cookie path used by the dragos profile.
//
//	go run ./cmd/dragos-cookie-probe          # human-readable listing
//	go run ./cmd/dragos-cookie-probe -value   # just the freshest cookie value, for `export T=$(...)`
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
)

func main() {
	valueOnly := flag.Bool("value", false, "print only the freshest cookie value (for shell substitution)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !*valueOnly {
		fmt.Println("Looking for dragos-auth-token cookies in all browser stores...")
	}

	cookies, err := kooky.ReadCookies(ctx, kooky.Name("dragos-auth-token"))
	if err != nil && !*valueOnly {
		fmt.Fprintf(os.Stderr, "kooky.ReadCookies error: %v\n", err)
	}
	if len(cookies) == 0 {
		if *valueOnly {
			fmt.Fprintln(os.Stderr, "no dragos-auth-token cookies found")
			os.Exit(1)
		}
		fmt.Println("No dragos-auth-token cookies found.")
		fmt.Println()
		fmt.Println("Possible reasons:")
		fmt.Println("  - macOS Keychain prompt appeared but wasn't approved (look behind other windows)")
		fmt.Println("  - Browser is using a profile kooky didn't auto-discover")
		fmt.Println("  - Cookie store file is locked (close the browser and retry)")
		os.Exit(1)
	}

	if *valueOnly {
		var best *kooky.Cookie
		for _, c := range cookies {
			if c == nil || c.Value == "" {
				continue
			}
			if best == nil || c.Expires.After(best.Expires) {
				best = c
			}
		}
		if best == nil {
			fmt.Fprintln(os.Stderr, "no usable dragos-auth-token cookie")
			os.Exit(1)
		}
		fmt.Println(best.Value)
		return
	}

	for i, c := range cookies {
		browser, profile, path := "", "", ""
		if c.Browser != nil {
			browser = c.Browser.Browser()
			profile = c.Browser.Profile()
			path = c.Browser.FilePath()
		}
		fmt.Printf("[%d] browser=%s profile=%s domain=%s secure=%v expires=%s\n",
			i, browser, profile, c.Domain, c.Secure, c.Expires.Format(time.RFC3339),
		)
		fmt.Printf("    store=%s\n", path)
		if len(c.Value) > 60 {
			fmt.Printf("    value(prefix)=%s...\n", c.Value[:60])
		} else {
			fmt.Printf("    value=%s\n", c.Value)
		}
	}
}
