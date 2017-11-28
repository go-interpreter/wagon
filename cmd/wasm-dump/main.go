// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-interpreter/wagon/wasm"
)

func main() {
	log.SetPrefix("wasm-dump: ")
	log.SetFlags(0)

	flagVerbose := flag.Bool("v", false, "enable/disable verbose mode")
	flagHeaders := flag.Bool("h", false, "print headers")
	// flagSection := flag.String("j", "", "select just one section")
	flagFull := flag.Bool("s", false, "print raw section contents") // TODO(sbinet)
	flagDis := flag.Bool("d", false, "disassemble function bodies") // TODO(sbinet)
	flagDetails := flag.Bool("x", false, "show section details")    // TODO(sbinet)

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		flag.PrintDefaults()
		os.Exit(1)
	}

	if !*flagHeaders && !*flagFull && !*flagDis && !*flagDetails {
		flag.Usage()
		flag.PrintDefaults()
		log.Printf("At least one of -d, -h, -x or -s must be given")
		os.Exit(1)
	}

	wasm.SetDebugMode(*flagVerbose)

	fname := flag.Arg(0)
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("could not open %q: %v", fname, err)
	}
	defer f.Close()

	m, err := wasm.ReadModule(f, nil)
	if err != nil {
		log.Fatalf("could not read module: %v", err)
	}

	if *flagHeaders {
		printHeaders(f.Name(), m)
	}
}

func printHeaders(fname string, m *wasm.Module) {
	fmt.Printf("%s: module version: %#x\n\n", fname, m.Version)
	fmt.Printf("sections:\n\n")

	hdrfmt := "%9s start=0x%08x end=0x%08x (size=0x%08x) count: %d\n"
	if sec := m.Types; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Import; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Function; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Types),
		)
	}
	if sec := m.Table; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Memory; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Global; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Globals),
		)
	}
	if sec := m.Export; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Elements; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	if sec := m.Code; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Bodies),
		)
	}
	if sec := m.Data; sec != nil {
		fmt.Printf(hdrfmt,
			sec.ID.String(),
			sec.Section.Start, sec.Section.End, sec.Section.PayloadLen,
			len(sec.Entries),
		)
	}
	for _, sec := range m.Other {
		fmt.Printf("%9s start=0x%08x end=0x%08x (size=0x%08x) %q\n",
			sec.ID.String(),
			sec.Start, sec.End, sec.PayloadLen,
			sec.Name,
		)
	}
}
