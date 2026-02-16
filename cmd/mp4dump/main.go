// Command mp4dump reads an MP4 file and prints its box structure.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tetsuo/mp4"
)

// Format specifies the output format.
type Format int

const (
	FormatText Format = iota
	FormatJSON
)

// BoxNode is a box in the tree structure.
type BoxNode struct {
	Type       string         `json:"type"`
	Size       uint64         `json:"size"`
	Version    *uint8         `json:"version,omitempty"`
	Flags      *uint32        `json:"flags,omitempty"`
	Info       map[string]any `json:"info,omitempty"`
	DataLength *int           `json:"dataLength,omitempty"`
	Children   []BoxNode      `json:"children,omitempty"`
}

func main() {
	formatFlag := flag.String("format", "text", "output format: text (default), json")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--format=text|json] <file.mp4>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	format := FormatText
	switch strings.ToLower(*formatFlag) {
	case "json":
		format = FormatJSON
	case "text":
		format = FormatText
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s\n", *formatFlag)
		os.Exit(1)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	// Build tree structure
	var root []BoxNode

	sc := mp4.NewScanner(f)
	for sc.Next() {
		e := sc.Entry()
		node := BoxNode{
			Type: string(e.Type[:]),
			Size: uint64(e.Size),
		}

		// Only load metadata boxes into memory for deep parsing
		if e.Type == mp4.TypeMoov || e.Type == mp4.TypeMoof {
			buf := make([]byte, e.DataSize())
			if err := sc.ReadBody(buf); err != nil {
				fmt.Fprintf(os.Stderr, "error reading %s: %v\n", e.Type, err)
				continue
			}
			r := mp4.NewReader(buf)
			node.Children = buildTree(&r)
		} else if e.Type == mp4.TypeFtyp {
			buf := make([]byte, e.DataSize())
			if err := sc.ReadBody(buf); err != nil {
				fmt.Fprintf(os.Stderr, "error reading ftyp: %v\n", err)
				continue
			}
			f := mp4.ReadFtyp(buf)
			node.Info = make(map[string]any)
			node.Info["brand"] = string(f.MajorBrand[:])
			node.Info["version"] = f.MinorVersion
			if len(f.Compatible) > 0 {
				compat := make([]string, len(f.Compatible))
				for i, c := range f.Compatible {
					compat[i] = string(c[:])
				}
				node.Info["compatible"] = compat
			}
		} else if e.Type == mp4.TypeMdat {
			dataLen := int(e.DataSize())
			node.DataLength = &dataLen
		}

		root = append(root, node)
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}

	printTree(root, format)
}

func buildTree(r *mp4.Reader) []BoxNode {
	var nodes []BoxNode

	for r.Next() {
		boxType := r.Type()
		node := BoxNode{
			Type: string(boxType[:]),
			Size: r.Size(),
		}

		if mp4.IsFullBox(r.Type()) {
			v := r.Version()
			f := r.Flags()
			node.Version = &v
			node.Flags = &f
		}

		// Collect box-specific info
		node.Info = collectBoxInfo(r)

		// Descend into containers
		if mp4.IsContainerBox(r.Type()) {
			r.Enter()
			node.Children = buildTree(r)
			r.Exit()
		} else if r.Type() == mp4.TypeStsd {
			// Handle stsd: entry count + sub-boxes
			r.Enter()
			r.Skip(4) // skip entry count
			for r.Next() {
				child := buildSampleEntryNode(r)
				node.Children = append(node.Children, child)
			}
			r.Exit()
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func buildSampleEntryNode(r *mp4.Reader) BoxNode {
	boxType := r.Type()
	node := BoxNode{
		Type: string(boxType[:]),
		Size: r.Size(),
		Info: make(map[string]any),
	}

	switch r.Type() {
	case mp4.TypeAvc1:
		v := mp4.ReadVisualSampleEntry(r.Data())
		node.Info["width"] = v.Width
		node.Info["height"] = v.Height
		node.Info["compressor"] = v.CompressorName

		// Enter to find avcC and other children
		r.Enter()
		r.Skip(v.ChildOffset)
		for r.Next() {
			childType := r.Type()
			child := BoxNode{
				Type: string(childType[:]),
				Size: r.Size(),
			}
			if mp4.IsFullBox(r.Type()) {
				ver := r.Version()
				flg := r.Flags()
				child.Version = &ver
				child.Flags = &flg
			}
			if r.Type() == mp4.TypeAvcC {
				codec := mp4.ReadAvcC(r.Data())
				child.Info = map[string]any{"codec": codec}
			}
			node.Children = append(node.Children, child)
		}
		r.Exit()

	case mp4.TypeMp4a:
		a := mp4.ReadAudioSampleEntry(r.Data())
		node.Info["channelCount"] = a.ChannelCount
		node.Info["sampleSize"] = a.SampleSize
		node.Info["sampleRate"] = a.SampleRate >> 16

		// Enter to find esds and other children
		r.Enter()
		r.Skip(a.ChildOffset)
		for r.Next() {
			childType := r.Type()
			child := BoxNode{
				Type: string(childType[:]),
				Size: r.Size(),
			}
			if mp4.IsFullBox(r.Type()) {
				ver := r.Version()
				flg := r.Flags()
				child.Version = &ver
				child.Flags = &flg
			}
			if r.Type() == mp4.TypeEsds {
				codec := mp4.ReadEsdsCodec(r.Data())
				child.Info = map[string]any{"codec": codec}
			}
			node.Children = append(node.Children, child)
		}
		r.Exit()

	default:
		if mp4.IsFullBox(r.Type()) {
			ver := r.Version()
			flg := r.Flags()
			node.Version = &ver
			node.Flags = &flg
		}
		dataLen := len(r.Data())
		node.DataLength = &dataLen
	}

	return node
}

func collectBoxInfo(r *mp4.Reader) map[string]any {
	info := make(map[string]any)

	switch r.Type() {
	case mp4.TypeFtyp:
		f := mp4.ReadFtyp(r.Data())
		info["brand"] = string(f.MajorBrand[:])
		info["version"] = f.MinorVersion
		if len(f.Compatible) > 0 {
			compat := make([]string, len(f.Compatible))
			for i, c := range f.Compatible {
				compat[i] = string(c[:])
			}
			info["compatible"] = compat
		}

	case mp4.TypeMvhd:
		ts, dur, ntid := r.ReadMvhd()
		info["timescale"] = ts
		info["duration"] = dur
		info["nextTrackId"] = ntid

	case mp4.TypeTkhd:
		tid, dur, w, h := r.ReadTkhd()
		info["trackId"] = tid
		info["duration"] = dur
		info["width"] = w >> 16
		info["height"] = h >> 16

	case mp4.TypeMdhd:
		ts, dur, lang := r.ReadMdhd()
		info["timescale"] = ts
		info["duration"] = dur
		info["language"] = lang

	case mp4.TypeHdlr:
		ht := r.ReadHdlr()
		name := r.ReadHdlrName()
		info["handlerType"] = string(ht[:])
		info["name"] = name

	case mp4.TypeStsd:
		if len(r.Data()) >= 4 {
			info["entries"] = r.EntryCount()
		}

	case mp4.TypeStsz:
		it := mp4.NewStszIter(r.Data())
		info["entries"] = it.Count()

	case mp4.TypeStco, mp4.TypeStss:
		it := mp4.NewUint32Iter(r.Data())
		info["entries"] = it.Count()

	case mp4.TypeCo64:
		it := mp4.NewCo64Iter(r.Data())
		info["entries"] = it.Count()

	case mp4.TypeStts:
		it := mp4.NewSttsIter(r.Data())
		info["entries"] = it.Count()

	case mp4.TypeCtts:
		it := mp4.NewCttsIter(r.Data(), r.Version())
		info["entries"] = it.Count()

	case mp4.TypeStsc:
		it := mp4.NewStscIter(r.Data())
		info["entries"] = it.Count()

	case mp4.TypeElst:
		it := mp4.NewElstIter(r.Data(), r.Version())
		info["entries"] = it.Count()

	case mp4.TypeDref:
		if len(r.Data()) >= 4 {
			info["entries"] = r.EntryCount()
		}

	case mp4.TypeMehd:
		dur := r.ReadMehd()
		info["fragmentDuration"] = dur

	case mp4.TypeTrex:
		tid, _, _, _, _ := r.ReadTrex()
		info["trackId"] = tid

	case mp4.TypeMfhd:
		seq := r.ReadMfhd()
		info["sequence"] = seq

	case mp4.TypeTfhd:
		tid := r.ReadTfhd()
		info["trackId"] = tid

	case mp4.TypeTfdt:
		bt := r.ReadTfdt()
		info["baseMediaDecodeTime"] = bt

	case mp4.TypeTrun:
		it := mp4.NewTrunIter(r.Data(), r.Flags())
		info["entries"] = it.Count()
		if r.Flags()&mp4.TrunDataOffsetPresent != 0 {
			info["dataOffset"] = it.DataOffset()
		}

	case mp4.TypeMdat:
		info["dataLength"] = len(r.Data())

	case mp4.TypeVmhd:
		// graphicsMode and opcolor
	case mp4.TypeSmhd:
		// balance
	default:
		if !mp4.IsContainerBox(r.Type()) {
			if len(r.Data()) > 0 {
				info["dataLength"] = len(r.Data())
			}
		}
	}

	return info
}

// printTree prints the tree in the specified format
func printTree(nodes []BoxNode, format Format) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(nodes); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		}
	case FormatText:
		for _, node := range nodes {
			printNodeText(node, 0)
		}
	}
}

// printNodeText prints a single node in text format
func printNodeText(node BoxNode, depth int) {
	indent := strings.Repeat("  ", depth)

	fmt.Printf("%s[%s] size=%d", indent, node.Type, node.Size)

	if node.Version != nil {
		fmt.Printf(" v=%d", *node.Version)
	}
	if node.Flags != nil {
		fmt.Printf(" flags=0x%06x", *node.Flags)
	}

	// Print info fields
	if len(node.Info) > 0 {
		for key, val := range node.Info {
			switch key {
			case "brand":
				fmt.Printf(" brand=%v", val)
			case "version":
				fmt.Printf(" ver=%v", val)
			case "compatible":
				if compat, ok := val.([]string); ok {
					fmt.Printf(" compat=[%s]", strings.Join(compat, ","))
				}
			case "timescale":
				fmt.Printf(" timescale=%v", val)
			case "duration":
				fmt.Printf(" duration=%v", val)
			case "nextTrackId":
				fmt.Printf(" nextTrackId=%v", val)
			case "trackId":
				fmt.Printf(" trackId=%v", val)
			case "width":
				if depth > 0 && node.Type == "avc1" {
					// Special handling for sample entries
					continue
				}
				fmt.Printf(" width=%v", val)
			case "height":
				if depth > 0 && node.Type == "avc1" {
					continue
				}
				fmt.Printf(" height=%v", val)
			case "language":
				fmt.Printf(" lang=%v", val)
			case "handlerType":
				fmt.Printf(" type=%v", val)
			case "name":
				fmt.Printf(" name=%q", val)
			case "entries":
				fmt.Printf(" entries=%v", val)
			case "fragmentDuration":
				fmt.Printf(" fragmentDuration=%v", val)
			case "sequence":
				fmt.Printf(" seq=%v", val)
			case "baseMediaDecodeTime":
				fmt.Printf(" baseMediaDecodeTime=%v", val)
			case "dataOffset":
				fmt.Printf(" dataOffset=%v", val)
			case "channelCount":
				fmt.Printf(" ch=%v", val)
			case "sampleSize":
				fmt.Printf(" sampleSize=%v", val)
			case "sampleRate":
				fmt.Printf(" sampleRate=%v", val)
			case "compressor":
				fmt.Printf(" compressor=%q", val)
			case "codec":
				fmt.Printf(" codec=%v", val)
			case "dataLength":
				// Skip, will be handled by DataLength field
			}
		}
		// Special formatting for avc1 and mp4a
		if node.Type == "avc1" {
			if w, haveW := node.Info["width"]; haveW {
				if h, haveH := node.Info["height"]; haveH {
					fmt.Printf(" %vx%v", w, h)
				}
			}
		}
	}

	if node.DataLength != nil {
		fmt.Printf(" dataLen=%d", *node.DataLength)
	}

	fmt.Println()

	// Print children
	for _, child := range node.Children {
		printNodeText(child, depth+1)
	}
}
