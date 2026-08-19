package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sshpie/milvscan/internal/scanner"
)

const usage = `milvscan -- Milvus unauth enumeration tool


usage: milvscan [flags] <target-url>

  target-url  http://host:19121  (Milvus v1 REST, default port)
              http://host:9530   (Milvus v2 REST)
              http://host:9091   (Milvus metrics/healthz)

flags:
  --probe              Fingerprint only: api-version, Milvus version, auth
  --hunt               Full enumeration (default): collections + schema + samples + PII
  --write-canary       Insert a write canary entity (also requires --authorize-write)
  --authorize-write    Explicit authorization gate for write operations
  --token <tok>        Bearer token for authorized authed-instance testing
  --limit <n>          Entity samples per collection (default: 3)
  --timeout <sec>      HTTP timeout (default: 10)
  -o <file>            Write JSON output to file
  -v                   Verbose: log HTTP exchanges

examples:
  milvscan http://target:19121
  milvscan http://target:9530 --probe
  milvscan http://target:19121 -o findings.json
  milvscan http://target:19121 --write-canary --authorize-write
  milvscan http://target:9530 --token SECRET
`

func main() {
	var (
		probeOnly  = flag.Bool("probe", false, "")
		doHunt     = flag.Bool("hunt", false, "")
		doCanary   = flag.Bool("write-canary", false, "")
		authWrite  = flag.Bool("authorize-write", false, "")
		token      = flag.String("token", "", "")
		limit      = flag.Int("limit", 3, "")
		timeout    = flag.Int("timeout", 10, "")
		outputFile = flag.String("o", "", "")
		verbose    = flag.Bool("v", false, "")
	)

	flag.Usage = func() { fmt.Print(usage) }

	// Iterative reparse so flags can appear after the positional target arg.
	var posArgs []string
	rest := os.Args[1:]
	for {
		if err := flag.CommandLine.Parse(rest); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		leftover := flag.Args()
		if len(leftover) == 0 {
			break
		}
		posArgs = append(posArgs, leftover[0])
		rest = leftover[1:]
	}

	if len(posArgs) == 0 {
		fmt.Print(usage)
		os.Exit(1)
	}

	if *doCanary && !*authWrite {
		fmt.Fprintln(os.Stderr, "[!] --write-canary requires --authorize-write")
		os.Exit(1)
	}

	target := posArgs[0]
	if !strings.HasPrefix(target, "http") {
		target = "http://" + target
	}
	target = strings.TrimRight(target, "/")

	_ = doHunt // --hunt is the default; --probe suppresses enumeration

	cfg := &scanner.Config{
		Target:        target,
		Token:         *token,
		TimeoutSec:    *timeout,
		DoWriteCanary: *doCanary && *authWrite,
		ProbeOnly:     *probeOnly,
		SampleLimit:   *limit,
		OutputFile:    *outputFile,
		Verbose:       *verbose,
	}

	result := &scanner.ScanResult{
		Target:   target,
		ScanTime: time.Now().UTC(),
	}

	s := scanner.New(cfg)
	if err := s.Run(result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	scanner.PrintReport(result)

	data, _ := json.MarshalIndent(result, "", "  ")
	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[+] JSON -> %s\n", *outputFile)
	}
}
