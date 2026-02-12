// Package mp4 implements encoding and decoding of ISO Base Media File Format (MP4) boxes.
package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var be = binary.BigEndian

const uint32Max = math.MaxUint32

// BoxType is a 4-byte box type identifier.
type BoxType [4]byte

func (t BoxType) String() string {
	return string(t[:])
}

// newBoxType creates a BoxType from a 4-character string.
func newBoxType(s string) BoxType {
	var t BoxType
	copy(t[:], s)
	return t
}

// Known box types.
var (
	TypeFtyp = newBoxType("ftyp")
	TypeMoov = newBoxType("moov")
	TypeMvhd = newBoxType("mvhd")
	TypeTrak = newBoxType("trak")
	TypeTkhd = newBoxType("tkhd")
	TypeTref = newBoxType("tref")
	TypeTrgr = newBoxType("trgr")
	TypeEdts = newBoxType("edts")
	TypeElst = newBoxType("elst")
	TypeMdia = newBoxType("mdia")
	TypeMdhd = newBoxType("mdhd")
	TypeHdlr = newBoxType("hdlr")
	TypeElng = newBoxType("elng")
	TypeMinf = newBoxType("minf")
	TypeVmhd = newBoxType("vmhd")
	TypeSmhd = newBoxType("smhd")
	TypeHmhd = newBoxType("hmhd")
	TypeSthd = newBoxType("sthd")
	TypeNmhd = newBoxType("nmhd")
	TypeDinf = newBoxType("dinf")
	TypeDref = newBoxType("dref")
	TypeStbl = newBoxType("stbl")
	TypeStsd = newBoxType("stsd")
	TypeStts = newBoxType("stts")
	TypeCtts = newBoxType("ctts")
	TypeCslg = newBoxType("cslg")
	TypeStsc = newBoxType("stsc")
	TypeStsz = newBoxType("stsz")
	TypeStz2 = newBoxType("stz2")
	TypeStco = newBoxType("stco")
	TypeCo64 = newBoxType("co64")
	TypeStss = newBoxType("stss")
	TypeStsh = newBoxType("stsh")
	TypePadb = newBoxType("padb")
	TypeStdp = newBoxType("stdp")
	TypeSdtp = newBoxType("sdtp")
	TypeSbgp = newBoxType("sbgp")
	TypeSgpd = newBoxType("sgpd")
	TypeSubs = newBoxType("subs")
	TypeSaiz = newBoxType("saiz")
	TypeSaio = newBoxType("saio")
	TypeMvex = newBoxType("mvex")
	TypeMehd = newBoxType("mehd")
	TypeTrex = newBoxType("trex")
	TypeLeva = newBoxType("leva")
	TypeMoof = newBoxType("moof")
	TypeMfhd = newBoxType("mfhd")
	TypeTraf = newBoxType("traf")
	TypeTfhd = newBoxType("tfhd")
	TypeTfdt = newBoxType("tfdt")
	TypeTrun = newBoxType("trun")
	TypeMeta = newBoxType("meta")
	TypeUdta = newBoxType("udta")
	TypeMdat = newBoxType("mdat")
	TypeAvc1 = newBoxType("avc1")
	TypeAvcC = newBoxType("avcC")
	TypeMp4a = newBoxType("mp4a")
	TypeEsds = newBoxType("esds")
)

// containerChild describes a child slot in a container box.
// If array is true, the key in the container is the plural form (e.g. "traks")
// and multiple children of this type can appear.
type containerChild struct {
	Type  BoxType
	Array bool
}

// containerDef defines which children a container box expects.
var containerDef = map[BoxType][]containerChild{
	TypeMoov: {{TypeMvhd, false}, {TypeMeta, false}, {TypeTrak, true}, {TypeMvex, false}},
	TypeTrak: {{TypeTkhd, false}, {TypeTref, false}, {TypeTrgr, false}, {TypeEdts, false}, {TypeMeta, false}, {TypeMdia, false}, {TypeUdta, false}},
	TypeEdts: {{TypeElst, false}},
	TypeMdia: {{TypeMdhd, false}, {TypeHdlr, false}, {TypeElng, false}, {TypeMinf, false}},
	TypeMinf: {{TypeVmhd, false}, {TypeSmhd, false}, {TypeHmhd, false}, {TypeSthd, false}, {TypeNmhd, false}, {TypeDinf, false}, {TypeStbl, false}},
	TypeDinf: {{TypeDref, false}},
	TypeStbl: {
		{TypeStsd, false}, {TypeStts, false}, {TypeCtts, false}, {TypeCslg, false},
		{TypeStsc, false}, {TypeStsz, false}, {TypeStz2, false}, {TypeStco, false},
		{TypeCo64, false}, {TypeStss, false}, {TypeStsh, false}, {TypePadb, false},
		{TypeStdp, false}, {TypeSdtp, false}, {TypeSbgp, true}, {TypeSgpd, true},
		{TypeSubs, true}, {TypeSaiz, true}, {TypeSaio, true},
	},
	TypeMvex: {{TypeMehd, false}, {TypeTrex, true}, {TypeLeva, false}},
	TypeMoof: {{TypeMfhd, false}, {TypeMeta, false}, {TypeTraf, true}},
	TypeTraf: {{TypeTfhd, false}, {TypeTfdt, false}, {TypeTrun, false}, {TypeSbgp, true}, {TypeSgpd, true}, {TypeSubs, true}, {TypeSaiz, true}, {TypeSaio, true}, {TypeMeta, false}},
}

// isContainer returns the container definition for a type, or nil.
func isContainer(t BoxType) []containerChild {
	return containerDef[t]
}

// fullBoxes is the set of box types that have version+flags in their header.
var fullBoxes = map[BoxType]bool{
	TypeMvhd: true, TypeTkhd: true, TypeMdhd: true, TypeVmhd: true, TypeSmhd: true,
	TypeStsd: true, TypeEsds: true, TypeStsz: true, TypeStco: true, TypeCo64: true,
	TypeStss: true, TypeStts: true, TypeCtts: true, TypeStsc: true, TypeDref: true,
	TypeElst: true, TypeHdlr: true, TypeMehd: true, TypeTrex: true, TypeMfhd: true,
	TypeTfhd: true, TypeTfdt: true, TypeTrun: true,
}

// Box represents an MP4 box (atom).
type Box struct {
	Type       BoxType
	Size       uint64 // total size including header
	Version    uint8
	Flags      uint32
	HasFullBox bool

	// Container children (keyed by type). For array children, use Children.
	Children map[BoxType][]*Box
	// OtherBoxes holds children not recognized by the container definition.
	OtherBoxes []*Box

	// Raw buffer for unknown box types.
	Buffer []byte

	// Typed payload: only one of these is non-nil for leaf boxes.
	Ftyp   *Ftyp
	Mvhd   *Mvhd
	Tkhd   *Tkhd
	Mdhd   *Mdhd
	Vmhd   *Vmhd
	Smhd   *Smhd
	Stsd   *Stsd
	Stsz   *Stsz
	Stco   *Stco // also used for stss
	Co64   *Co64
	Stts   *Stts
	Ctts   *Ctts
	Stsc   *Stsc
	Dref   *DrefBox
	Elst   *Elst
	Hdlr   *Hdlr
	Mehd   *Mehd
	Trex   *Trex
	Mfhd   *Mfhd
	Tfhd   *Tfhd
	Tfdt   *Tfdt
	Trun   *Trun
	Mdat   *Mdat
	AvcC   *AvcC
	Visual *VisualSampleEntry
	Audio  *AudioSampleEntry
	Esds   *Esds
}

// Child returns the first child box of the given type, or nil.
func (b *Box) Child(t BoxType) *Box {
	if b.Children == nil {
		return nil
	}
	cs := b.Children[t]
	if len(cs) == 0 {
		return nil
	}
	return cs[0]
}

// ChildList returns all child boxes of the given type.
func (b *Box) ChildList(t BoxType) []*Box {
	if b.Children == nil {
		return nil
	}
	return b.Children[t]
}

// Headers holds parsed box header information.
type Headers struct {
	Size       uint64
	HeaderSize int
	ContentLen int
	Type       BoxType
	Version    uint8
	Flags      uint32
}

// ReadHeaders parses box headers from buf[start:end].
// Returns the headers and nil error, or if not enough data, returns an error.
func ReadHeaders(buf []byte, start, end int) (Headers, error) {
	if end-start < 8 {
		return Headers{}, errors.New("need at least 8 bytes")
	}

	size := uint64(be.Uint32(buf[start:]))
	var t BoxType
	copy(t[:], buf[start+4:])
	ptr := start + 8

	if size == 1 {
		if end-start < 16 {
			return Headers{}, errors.New("need at least 16 bytes for extended size")
		}
		size = be.Uint64(buf[ptr:])
		ptr += 8
	}

	var version uint8
	var flags uint32
	if fullBoxes[t] {
		if end-ptr < 4 {
			return Headers{}, errors.New("need 4 more bytes for full box header")
		}
		vf := be.Uint32(buf[ptr:])
		version = uint8(vf >> 24)
		flags = vf & 0x00ffffff
		ptr += 4
	}

	hdrSize := ptr - start
	return Headers{
		Size:       size,
		HeaderSize: hdrSize,
		ContentLen: int(size) - hdrSize,
		Type:       t,
		Version:    version,
		Flags:      flags,
	}, nil
}

// Decode decodes a box from buf[start:end].
func Decode(buf []byte, start, end int) (*Box, error) {
	h, err := ReadHeaders(buf, start, end)
	if err != nil {
		return nil, err
	}
	if int(h.Size) > end-start {
		return nil, fmt.Errorf("box %s: data too short (need %d, have %d)", h.Type, h.Size, end-start)
	}
	return decodeBody(h, buf, start+h.HeaderSize, start+int(h.Size))
}

func decodeBody(h Headers, buf []byte, start, end int) (*Box, error) {
	box := &Box{
		Type:       h.Type,
		Size:       h.Size,
		Version:    h.Version,
		Flags:      h.Flags,
		HasFullBox: fullBoxes[h.Type],
	}

	if def := isContainer(h.Type); def != nil {
		box.Children = make(map[BoxType][]*Box)
		// Build lookup: which types are expected and whether they're arrays
		expected := make(map[BoxType]bool, len(def)) // true = array
		for _, c := range def {
			expected[c.Type] = c.Array
		}

		ptr := start
		for end-ptr >= 8 {
			child, err := Decode(buf, ptr, end)
			if err != nil {
				return nil, fmt.Errorf("in container %s: %w", h.Type, err)
			}
			ptr += int(child.Size)

			isArr, known := expected[child.Type]
			if known {
				if isArr {
					box.Children[child.Type] = append(box.Children[child.Type], child)
				} else {
					box.Children[child.Type] = []*Box{child}
				}
			} else {
				box.OtherBoxes = append(box.OtherBoxes, child)
			}
		}
	} else if codec := getCodec(h.Type); codec != nil {
		if err := codec.decode(box, buf, start, end); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", h.Type, err)
		}
	} else {
		// Unknown box: store raw data
		box.Buffer = make([]byte, end-start)
		copy(box.Buffer, buf[start:end])
	}

	return box, nil
}

// EncodingLength computes the total encoded size of the box; populates Size fields.
func EncodingLength(box *Box) uint64 {
	var size uint64 = 8
	if fullBoxes[box.Type] {
		size += 4
	}

	if def := isContainer(box.Type); def != nil {
		for _, cd := range def {
			children := box.Children[cd.Type]
			for _, child := range children {
				child.Type = cd.Type
				size += EncodingLength(child)
			}
		}
		for _, child := range box.OtherBoxes {
			size += EncodingLength(child)
		}
	} else if codec := getCodec(box.Type); codec != nil {
		size += uint64(codec.encodingLength(box))
	} else if box.Buffer != nil {
		size += uint64(len(box.Buffer))
	}

	if size > uint32Max {
		size += 8 // extended size field
	}

	box.Size = size
	return size
}

// Encode encodes the box into buf starting at offset.
// Returns the number of bytes written.
func Encode(box *Box, buf []byte, offset int) (int, error) {
	EncodingLength(box)
	return encodeBox(box, buf, offset)
}

func encodeBox(box *Box, buf []byte, offset int) (int, error) {
	sz := box.Size
	hdrSz := uint32(sz)
	if sz > uint32Max {
		hdrSz = 1
	}

	be.PutUint32(buf[offset:], hdrSz)
	copy(buf[offset+4:], box.Type[:])
	ptr := offset + 8

	if hdrSz == 1 {
		be.PutUint64(buf[ptr:], sz)
		ptr += 8
	}

	if fullBoxes[box.Type] {
		vf := (uint32(box.Version) << 24) | (box.Flags & 0x00ffffff)
		be.PutUint32(buf[ptr:], vf)
		ptr += 4
	}

	if def := isContainer(box.Type); def != nil {
		for _, cd := range def {
			children := box.Children[cd.Type]
			for _, child := range children {
				n, err := encodeBox(child, buf, ptr)
				if err != nil {
					return 0, err
				}
				ptr += n
			}
		}
		for _, child := range box.OtherBoxes {
			n, err := encodeBox(child, buf, ptr)
			if err != nil {
				return 0, err
			}
			ptr += n
		}
	} else if codec := getCodec(box.Type); codec != nil {
		n := codec.encode(box, buf, ptr)
		ptr += n
	} else if box.Buffer != nil {
		copy(buf[ptr:], box.Buffer)
		ptr += len(box.Buffer)
	}

	return ptr - offset, nil
}

// EncodeToBytes is a convenience that allocates a buffer and encodes the box.
func EncodeToBytes(box *Box) ([]byte, error) {
	sz := EncodingLength(box)
	buf := make([]byte, sz)
	_, err := encodeBox(box, buf, 0)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// EncodeToBuf encodes the box into the provided buffer slice.
// The buffer will be grown if needed. Returns the slice containing the encoded data.
func EncodeToBuf(box *Box, buf []byte) ([]byte, error) {
	sz := int(EncodingLength(box))
	if cap(buf) < sz {
		buf = make([]byte, sz)
	} else {
		buf = buf[:sz]
	}
	_, err := encodeBox(box, buf, 0)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func readString(buf []byte, offset, maxLen int) string {
	end := offset
	limit := offset + maxLen
	if limit > len(buf) {
		limit = len(buf)
	}
	for end < limit && buf[end] != 0 {
		end++
	}
	return string(buf[offset:end])
}

func clearBytes(buf []byte, start, end int) {
	clear(buf[start:end])
}
