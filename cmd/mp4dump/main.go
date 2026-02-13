// Command mp4dump reads an MP4 file and prints its box structure.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tetsuo/mp4"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.mp4>\n", os.Args[0])
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	sc := mp4.NewScanner(f)
	for sc.Next() {
		e := sc.Entry()
		fmt.Printf("[%s] size=%d\n", e.Type, e.Size)

		// Only load metadata boxes into memory for deep parsing
		if e.Type == mp4.TypeMoov || e.Type == mp4.TypeMoof {
			buf := make([]byte, e.DataSize())
			if err := sc.ReadBody(buf); err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", e.Type, err)
				continue
			}
			r := mp4.NewReader(buf)
			walk(&r, 1)
		} else if e.Type == mp4.TypeFtyp {
			buf := make([]byte, e.DataSize())
			if err := sc.ReadBody(buf); err != nil {
				fmt.Fprintf(os.Stderr, "error reading ftyp: %v\n", err)
				continue
			}
			f := mp4.ReadFtyp(buf)
			fmt.Printf("  brand=%s ver=%d", string(f.MajorBrand[:]), f.MinorVersion)
			if len(f.Compatible) > 0 {
				fmt.Printf(" compat=[")
				for i, c := range f.Compatible {
					if i > 0 {
						fmt.Printf(",")
					}
					fmt.Printf("%s", string(c[:]))
				}
				fmt.Printf("]")
			}
			fmt.Println()
		} else if e.Type == mp4.TypeMdat {
			fmt.Printf("  dataLen=%d\n", e.DataSize())
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}
}

func walk(r *mp4.Reader, depth int) {
	for r.Next() {
		indent := strings.Repeat("  ", depth)

		fmt.Printf("%s[%s] size=%d", indent, r.Type(), r.Size())

		if mp4.IsFullBox(r.Type()) {
			fmt.Printf(" v=%d flags=0x%06x", r.Version(), r.Flags())
		}

		printBoxInfo(r)
		fmt.Println()

		// Descend into containers
		if mp4.IsContainerBox(r.Type()) {
			r.Enter()
			walk(r, depth+1)
			r.Exit()
			continue
		}

		// Handle stsd: entry count + sub-boxes
		if r.Type() == mp4.TypeStsd {
			r.Enter()
			r.Skip(4) // skip entry count
			for r.Next() {
				printSampleEntry(r, depth+1)
			}
			r.Exit()
			continue
		}
	}
}

func printSampleEntry(r *mp4.Reader, depth int) {
	indent := strings.Repeat("  ", depth)

	switch r.Type() {
	case mp4.TypeAvc1:
		v := mp4.ReadVisualSampleEntry(r.Data())
		fmt.Printf("%s[%s] size=%d %dx%d compressor=%q\n", indent, r.Type(), r.Size(), v.Width, v.Height, v.CompressorName)
		// Enter to find avcC and other children
		r.Enter()
		r.Skip(v.ChildOffset)
		for r.Next() {
			childIndent := strings.Repeat("  ", depth+1)
			if mp4.IsFullBox(r.Type()) {
				fmt.Printf("%s[%s] size=%d v=%d flags=0x%06x", childIndent, r.Type(), r.Size(), r.Version(), r.Flags())
			} else {
				fmt.Printf("%s[%s] size=%d", childIndent, r.Type(), r.Size())
			}
			if r.Type() == mp4.TypeAvcC {
				codec := mp4.ReadAvcC(r.Data())
				fmt.Printf(" codec=%s", codec)
			}
			fmt.Println()
		}
		r.Exit()

	case mp4.TypeMp4a:
		a := mp4.ReadAudioSampleEntry(r.Data())
		fmt.Printf("%s[%s] size=%d ch=%d sampleSize=%d sampleRate=%d\n", indent, r.Type(), r.Size(), a.ChannelCount, a.SampleSize, a.SampleRate>>16)
		// Enter to find esds and other children
		r.Enter()
		r.Skip(a.ChildOffset)
		for r.Next() {
			childIndent := strings.Repeat("  ", depth+1)
			if mp4.IsFullBox(r.Type()) {
				fmt.Printf("%s[%s] size=%d v=%d flags=0x%06x", childIndent, r.Type(), r.Size(), r.Version(), r.Flags())
			} else {
				fmt.Printf("%s[%s] size=%d", childIndent, r.Type(), r.Size())
			}
			if r.Type() == mp4.TypeEsds {
				codec := mp4.ReadEsdsCodec(r.Data())
				fmt.Printf(" codec=%s", codec)
			}
			fmt.Println()
		}
		r.Exit()

	default:
		fmt.Printf("%s[%s] size=%d", indent, r.Type(), r.Size())
		if mp4.IsFullBox(r.Type()) {
			fmt.Printf(" v=%d flags=0x%06x", r.Version(), r.Flags())
		}
		fmt.Printf(" (raw %d bytes)\n", len(r.Data()))
	}
}

func printBoxInfo(r *mp4.Reader) {
	switch r.Type() {
	case mp4.TypeFtyp:
		f := mp4.ReadFtyp(r.Data())
		fmt.Printf(" brand=%s ver=%d", string(f.MajorBrand[:]), f.MinorVersion)
		if len(f.Compatible) > 0 {
			fmt.Printf(" compat=[")
			for i, c := range f.Compatible {
				if i > 0 {
					fmt.Printf(",")
				}
				fmt.Printf("%s", string(c[:]))
			}
			fmt.Printf("]")
		}

	case mp4.TypeMvhd:
		ts, dur, ntid := r.ReadMvhd()
		fmt.Printf(" timescale=%d duration=%d nextTrackId=%d", ts, dur, ntid)

	case mp4.TypeTkhd:
		tid, dur, w, h := r.ReadTkhd()
		fmt.Printf(" trackId=%d duration=%d size=%dx%d", tid, dur, w>>16, h>>16)

	case mp4.TypeMdhd:
		ts, dur, lang := r.ReadMdhd()
		fmt.Printf(" timescale=%d duration=%d lang=%d", ts, dur, lang)

	case mp4.TypeHdlr:
		ht := r.ReadHdlr()
		name := r.ReadHdlrName()
		fmt.Printf(" type=%s name=%q", string(ht[:]), name)

	case mp4.TypeStsd:
		if len(r.Data()) >= 4 {
			fmt.Printf(" entries=%d", r.EntryCount())
		}

	case mp4.TypeStsz:
		it := mp4.NewStszIter(r.Data())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeStco, mp4.TypeStss:
		it := mp4.NewUint32Iter(r.Data())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeCo64:
		it := mp4.NewCo64Iter(r.Data())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeStts:
		it := mp4.NewSttsIter(r.Data())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeCtts:
		it := mp4.NewCttsIter(r.Data(), r.Version())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeStsc:
		it := mp4.NewStscIter(r.Data())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeElst:
		it := mp4.NewElstIter(r.Data(), r.Version())
		fmt.Printf(" entries=%d", it.Count())

	case mp4.TypeDref:
		if len(r.Data()) >= 4 {
			fmt.Printf(" entries=%d", r.EntryCount())
		}

	case mp4.TypeMehd:
		dur := r.ReadMehd()
		fmt.Printf(" fragmentDuration=%d", dur)

	case mp4.TypeTrex:
		tid, _, _, _, _ := r.ReadTrex()
		fmt.Printf(" trackId=%d", tid)

	case mp4.TypeMfhd:
		seq := r.ReadMfhd()
		fmt.Printf(" seq=%d", seq)

	case mp4.TypeTfhd:
		tid := r.ReadTfhd()
		fmt.Printf(" trackId=%d", tid)

	case mp4.TypeTfdt:
		bt := r.ReadTfdt()
		fmt.Printf(" baseMediaDecodeTime=%d", bt)

	case mp4.TypeTrun:
		it := mp4.NewTrunIter(r.Data(), r.Flags())
		fmt.Printf(" entries=%d", it.Count())
		if r.Flags()&mp4.TrunDataOffsetPresent != 0 {
			fmt.Printf(" dataOffset=%d", it.DataOffset())
		}

	case mp4.TypeMdat:
		fmt.Printf(" dataLen=%d", len(r.Data()))

	case mp4.TypeVmhd:
		// graphicsMode and opcolor
	case mp4.TypeSmhd:
		// balance
	default:
		if !mp4.IsContainerBox(r.Type()) {
			if len(r.Data()) > 0 {
				fmt.Printf(" (%d bytes)", len(r.Data()))
			}
		}
	}
}
