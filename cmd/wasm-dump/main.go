// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/wasm"
	"github.com/go-interpreter/wagon/wasm/leb128"
)

// TODO: track the number of imported funcs,memories,tables and globals to adjust
// for their index offset when printing sections' content.

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: wasm-dump [options] file1.wasm [file2.wasm [...]]

ex:
 $> wasm-dump -h ./file1.wasm

options:
`,
		)
		flag.PrintDefaults()
		os.Exit(1)
	}
}

var (
	flagVerbose = flag.Bool("v", false, "enable/disable verbose mode")
	flagHeaders = flag.Bool("h", false, "print headers")
	// flagSection = flag.String("j", "", "select just one section")
	flagFull    = flag.Bool("s", false, "print raw section contents")
	flagDis     = flag.Bool("d", false, "disassemble function bodies")
	flagDetails = flag.Bool("x", false, "show section details")
)

func main() {
	log.SetPrefix("wasm-dump: ")
	log.SetFlags(0)

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if !*flagHeaders && !*flagFull && !*flagDis && !*flagDetails {
		flag.Usage()
		flag.PrintDefaults()
		log.Printf("At least one of -d, -h, -x or -s must be given")
		os.Exit(1)
	}

	wasm.SetDebugMode(*flagVerbose)

	w := os.Stdout
	for i, fname := range flag.Args() {
		if i > 0 {
			fmt.Fprintf(w, "\n")
		}
		process(w, fname)
	}
}

func process(w io.Writer, fname string) {
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
		printHeaders(w, f.Name(), m)
	}
	if *flagFull {
		printFull(w, f.Name(), m)
	}
	if *flagDis {
		printDis(w, f.Name(), m)
	}
	if *flagDetails {
		printDetails(w, f.Name(), m)
	}
}

func printHeaders(w io.Writer, fname string, m *wasm.Module) {
	fmt.Fprintf(w, "%s: module version: %#x\n\n", fname, m.Version)
	fmt.Fprintf(w, "sections:\n\n")

	hdrfmt := "%9s start=0x%08x end=0x%08x (size=0x%08x) count: %d\n"
	if sec := m.Types; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Import; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Function; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Types),
		)
	}
	if sec := m.Table; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Memory; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Global; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Globals),
		)
	}
	if sec := m.Export; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Start; sec != nil {
		hdrfmt := "%9s start=0x%08x end=0x%08x (size=0x%08x) start: %d\n"
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			sec.Index,
		)
	}
	if sec := m.Elements; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	if sec := m.Code; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Bodies),
		)
	}
	if sec := m.Data; sec != nil {
		fmt.Fprintf(w, hdrfmt,
			sec.ID.String(),
			sec.RawSection.Start, sec.RawSection.End, len(sec.RawSection.Bytes),
			len(sec.Entries),
		)
	}
	for _, sec := range m.Customs {
		fmt.Fprintf(w, "%9s start=0x%08x end=0x%08x (size=0x%08x) %q\n",
			sec.ID.String(),
			sec.Start, sec.End, len(sec.Bytes),
			sec.Name,
		)
	}
}

func printFull(w io.Writer, fname string, m *wasm.Module) {
	fmt.Fprintf(w, "%s: module version: %#x\n\n", fname, m.Version)

	hdrfmt := "contents of section %s:\n"
	sections := m.Sections

	for _, sec := range sections {
		rs := sec.GetRawSection()
		fmt.Fprintf(w, hdrfmt, rs.ID.String())
		fmt.Fprintln(w, hexDump(rs.Bytes, uint(rs.Start)))
	}
}

func printDis(w io.Writer, fname string, m *wasm.Module) {
	fmt.Fprintf(w, "%s: module version: %#x\n\n", fname, m.Version)
	fmt.Fprintf(w, "code disassembly:\n")
	for i := range m.Function.Types {
		f := m.GetFunction(i)
		fmt.Fprintf(w, "\nfunc[%d]: %v\n", i, f.Sig)
		dis, err := disasm.NewDisassembly(*f, m)
		if err != nil {
			log.Fatal(err)
		}
		offset := 0
		for _, code := range dis.Code {
			n := 1
			buf := new(bytes.Buffer)
			str := new(bytes.Buffer)
			fmt.Fprintf(buf, "%02x", code.Op.Code)
			fmt.Fprintf(str, "%v", code.Op.Name)
			for _, im := range code.Immediates {
				imbuf := new(bytes.Buffer)
				binary.Write(imbuf, binary.LittleEndian, im)
				n += imbuf.Len() / 2
				for _, cc := range imbuf.Bytes() {
					fmt.Fprintf(buf, " %02x", cc)
				}
				fmt.Fprintf(str, " %v", im)
			}
			fmt.Fprintf(w, " %06x: %-26s | %s\n", offset, buf.String(), str.String())
			offset += 2 * n
		}
		fmt.Fprintf(w, " %06x: %-26s | %s\n", offset, fmt.Sprintf("%02x", 0xb), "end")
	}
}

func printDetails(w io.Writer, fname string, m *wasm.Module) {
	fmt.Fprintf(w, "%s: module version: %#x\n\n", fname, m.Version)
	fmt.Fprintf(w, "section details:\n\n")

	if sec := m.Types; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, f := range sec.Entries {
			fmt.Fprintf(w, " - type[%d] %v\n", i, f)
		}
	}
	if sec := m.Import; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, e := range sec.Entries {
			buf := new(bytes.Buffer)
			switch typ := e.Type.(type) {
			case wasm.GlobalVarImport:
				fmt.Fprintf(buf, "%s mutable=%v",
					typ.Type.Type,
					typ.Type.Mutable,
				)
			case wasm.FuncImport:
				fmt.Fprintf(buf, "sig=%v", typ.Type)
			case wasm.MemoryImport:
				fmt.Fprintf(buf, "pages: initial=%d max=%d",
					typ.Type.Limits.Initial,
					typ.Type.Limits.Maximum,
				)
			case wasm.TableImport:
				fmt.Fprintf(buf, "elem_type=%v init=%v max=%v",
					typ.Type.ElementType,
					typ.Type.Limits.Initial,
					typ.Type.Limits.Maximum,
				)
			}
			fmt.Fprintf(w, " - %v[%d] %s <- %s.%s\n",
				e.Type.Kind(), i, buf.String(), e.ModuleName, e.FieldName,
			)
		}
	}
	if sec := m.Function; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, t := range sec.Types {
			fmt.Fprintf(w, " - func[%d] sig=%d\n", i, t)
		}
	}
	if sec := m.Table; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, e := range sec.Entries {
			fmt.Fprintf(w, " - table[%d] type=%v initial=%v\n", i, e.ElementType, e.Limits.Initial)
		}
	}
	if sec := m.Memory; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, e := range sec.Entries {
			fmt.Fprintf(w, " - memory[%d] pages: initial=%v\n", i, e.Limits.Initial)
		}
	}
	if sec := m.Global; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, g := range sec.Globals {
			// TODO(sbinet) display init infos
			fmt.Fprintf(w, " - global[%d] %v mutable=%v -- init: %#v\n", i, g.Type.Type, g.Type.Mutable, g.Init)
		}
	}
	if sec := m.Export; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		keys := make([]string, 0, len(sec.Entries))
		for n := range sec.Entries {
			keys = append(keys, n)
		}
		sort.Strings(keys)
		for _, name := range keys {
			e := sec.Entries[name]
			fmt.Fprintf(w, " - %v[%d] -> %q\n", e.Kind, e.Index, name)
		}
	}
	if sec := m.Start; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		fmt.Fprintf(w, " - start function: %d\n", sec.Index)
	}
	if sec := m.Elements; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, e := range sec.Entries {
			fmt.Fprintf(w, " - segment[%d] table=%d\n", i, e.Index)
			fmt.Fprintf(w, " - init: %#v\n", e.Offset)
			for ii, elem := range e.Elems {
				fmt.Fprintf(w, "  - elem[%d] = func[%d]\n", ii, elem)
			}
		}
	}
	if sec := m.Data; sec != nil {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		for i, e := range sec.Entries {
			fmt.Fprintf(w, " - segment[%d] size=%d - init %#v\n", i, len(e.Data), e.Offset)
			fmt.Fprintf(w, "%s", hexDump(e.Data, 0))
		}
	}
	for _, sec := range m.Customs {
		fmt.Fprintf(w, "%v:\n", sec.ID)
		fmt.Fprintf(w, " - name: %q\n", sec.Name)
		raw := bytes.NewReader(sec.Bytes[6:])
		for {
			if raw.Len() == 0 {
				break
			}
			i, err := leb128.ReadVarUint32(raw)
			if err != nil {
				log.Fatal(err)
			}
			n, err := leb128.ReadVarUint32(raw)
			if err != nil {
				log.Fatal(err)
			}
			str := make([]byte, int(n))
			_, err = io.ReadFull(raw, str)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(w, " - func[%d] %v\n", i, string(str))
		}
	}
}
